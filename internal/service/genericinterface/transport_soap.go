package genericinterface

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/models"
)

// SOAPTransport implements the Transport interface for SOAP web services.
type SOAPTransport struct {
	client *http.Client
}

// NewSOAPTransport creates a new SOAP transport.
func NewSOAPTransport() *SOAPTransport {
	return &SOAPTransport{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Type returns the transport type.
func (t *SOAPTransport) Type() string {
	return "HTTP::SOAP"
}

// SOAPEnvelope represents the SOAP envelope structure.
type SOAPEnvelope struct {
	XMLName xml.Name    `xml:"http://schemas.xmlsoap.org/soap/envelope/ Envelope"`
	Header  *SOAPHeader `xml:"Header,omitempty"`
	Body    SOAPBody    `xml:"Body"`
}

// SOAPHeader represents the SOAP header.
type SOAPHeader struct {
	Content []byte `xml:",innerxml"`
}

// SOAPBody represents the SOAP body.
type SOAPBody struct {
	Content []byte `xml:",innerxml"`
	Fault   *SOAPFault
}

// SOAPFault represents a SOAP fault response.
type SOAPFault struct {
	XMLName     xml.Name `xml:"http://schemas.xmlsoap.org/soap/envelope/ Fault"`
	FaultCode   string   `xml:"faultcode"`
	FaultString string   `xml:"faultstring"`
	Detail      string   `xml:"detail,omitempty"`
}

// Error implements the error interface for SOAPFault.
func (f *SOAPFault) Error() string {
	return fmt.Sprintf("SOAP Fault: %s - %s", f.FaultCode, f.FaultString)
}

// Execute performs a SOAP request to the remote service.
func (t *SOAPTransport) Execute(ctx context.Context, config models.TransportHTTPConfig, request *Request) (*Response, error) {
	// Build SOAP envelope
	soapBody, err := t.buildSOAPBody(config, request)
	if err != nil {
		return nil, fmt.Errorf("failed to build SOAP body: %w", err)
	}

	envelope := SOAPEnvelope{
		Body: SOAPBody{
			Content: soapBody,
		},
	}

	// Marshal envelope to XML
	xmlData, err := xml.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal SOAP envelope: %w", err)
	}

	// Add XML declaration
	xmlPayload := []byte(xml.Header + string(xmlData))

	// Determine endpoint URL
	endpoint := config.Host
	if config.Endpoint != "" {
		endpoint = strings.TrimSuffix(endpoint, "/") + "/" + strings.TrimPrefix(config.Endpoint, "/")
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(xmlPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set SOAP headers
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	if config.SOAPAction != "" {
		req.Header.Set("SOAPAction", config.SOAPAction)
	} else if request.Operation != "" {
		// Use operation name as SOAPAction if not configured
		namespace := config.NameSpace
		if namespace == "" {
			namespace = "http://tempuri.org/"
		}
		req.Header.Set("SOAPAction", namespace+request.Operation)
	}

	// Add additional headers
	for k, v := range config.AdditionalHeaders {
		req.Header.Set(k, v)
	}

	// Apply authentication
	if err := t.applyAuth(req, config.Authentication); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Set timeout from config
	if config.Timeout != "" {
		timeout, err := time.ParseDuration(config.Timeout + "s")
		if err == nil {
			t.client.Timeout = timeout
		}
	}

	// Execute request
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Build response
	response := &Response{
		StatusCode: resp.StatusCode,
		RawData:    respBody,
		Headers:    make(map[string]string),
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
	}

	// Copy response headers
	for k := range resp.Header {
		response.Headers[k] = resp.Header.Get(k)
	}

	// Parse SOAP response
	if len(respBody) > 0 {
		parsedData, fault, err := t.parseSOAPResponse(respBody, config)
		if err != nil {
			response.Error = fmt.Sprintf("Failed to parse SOAP response: %v", err)
			response.Success = false
		} else if fault != nil {
			response.Error = fault.Error()
			response.Success = false
		} else {
			response.Data = parsedData
		}
	}

	if !response.Success && response.Error == "" {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return response, nil
}

// buildSOAPBody creates the SOAP body XML for the request.
func (t *SOAPTransport) buildSOAPBody(config models.TransportHTTPConfig, request *Request) ([]byte, error) {
	namespace := config.NameSpace
	if namespace == "" {
		namespace = "http://tempuri.org/"
	}

	var buf bytes.Buffer

	// Start operation element with namespace
	operationName := request.Operation
	buf.WriteString(fmt.Sprintf(`<%s xmlns="%s">`, operationName, namespace))

	// Add parameters from request data
	for key, value := range request.Data {
		buf.WriteString(fmt.Sprintf("<%s>", key))
		buf.WriteString(t.escapeXML(fmt.Sprintf("%v", value)))
		buf.WriteString(fmt.Sprintf("</%s>", key))
	}

	// Close operation element
	buf.WriteString(fmt.Sprintf("</%s>", operationName))

	return buf.Bytes(), nil
}

// parseSOAPResponse parses a SOAP response envelope.
func (t *SOAPTransport) parseSOAPResponse(data []byte, config models.TransportHTTPConfig) (map[string]interface{}, *SOAPFault, error) {
	dataStr := string(data)

	// Check for SOAP fault first (can appear with different namespace prefixes)
	if strings.Contains(dataStr, "Fault") && (strings.Contains(dataStr, "faultcode") || strings.Contains(dataStr, "faultstring")) {
		fault := &SOAPFault{}
		fault.FaultCode = t.extractXMLValue(dataStr, "faultcode")
		fault.FaultString = t.extractXMLValue(dataStr, "faultstring")
		fault.Detail = t.extractXMLValue(dataStr, "detail")
		if fault.FaultCode != "" || fault.FaultString != "" {
			return nil, fault, nil
		}
	}

	// Try to parse as a SOAP envelope
	var envelope SOAPEnvelope
	if err := xml.Unmarshal(data, &envelope); err != nil {
		// Try to parse the raw XML as data
		return t.parseXMLToMap(data)
	}

	// Parse body content as data
	return t.parseXMLToMap(envelope.Body.Content)
}

// extractXMLValue extracts text content from an XML element, handling namespaced tags.
func (t *SOAPTransport) extractXMLValue(xmlStr, elementName string) string {
	// Try with and without namespace prefix
	patterns := []string{
		"<" + elementName + ">",
		"<soap:" + elementName + ">",
		"<SOAP-ENV:" + elementName + ">",
	}

	for _, startTag := range patterns {
		startIdx := strings.Index(xmlStr, startTag)
		if startIdx == -1 {
			continue
		}
		startIdx += len(startTag)

		// Find end tag (with or without prefix)
		endPatterns := []string{
			"</" + elementName + ">",
			"</soap:" + elementName + ">",
			"</SOAP-ENV:" + elementName + ">",
		}

		for _, endTag := range endPatterns {
			endIdx := strings.Index(xmlStr[startIdx:], endTag)
			if endIdx != -1 {
				return strings.TrimSpace(xmlStr[startIdx : startIdx+endIdx])
			}
		}
	}

	return ""
}

// parseXMLToMap converts XML to a map structure.
func (t *SOAPTransport) parseXMLToMap(data []byte) (map[string]interface{}, *SOAPFault, error) {
	result := make(map[string]interface{})

	decoder := xml.NewDecoder(bytes.NewReader(data))
	var currentKey string
	var stack []string

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}

		switch t := token.(type) {
		case xml.StartElement:
			stack = append(stack, t.Name.Local)
			currentKey = t.Name.Local
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if len(stack) > 0 {
				currentKey = stack[len(stack)-1]
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" && currentKey != "" {
				// Build nested key path
				if len(stack) == 1 {
					result[currentKey] = text
				} else if len(stack) > 1 {
					// For nested elements, store with full path or just the leaf
					result[currentKey] = text
				}
			}
		}
	}

	return result, nil, nil
}

// escapeXML escapes special characters for XML.
func (t *SOAPTransport) escapeXML(s string) string {
	var buf bytes.Buffer
	xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// applyAuth applies authentication to the request.
func (t *SOAPTransport) applyAuth(req *http.Request, auth models.AuthConfig) error {
	switch auth.AuthType {
	case "BasicAuth":
		if auth.BasicAuthUser != "" {
			req.SetBasicAuth(auth.BasicAuthUser, auth.BasicAuthPassword)
		}
	case "APIKey":
		if auth.APIKey != "" {
			header := auth.APIKeyHeader
			if header == "" {
				header = "X-API-Key"
			}
			req.Header.Set(header, auth.APIKey)
		}
	case "":
		// No authentication
	}
	return nil
}

// TestConnection tests connectivity to the SOAP endpoint.
func (t *SOAPTransport) TestConnection(ctx context.Context, config models.TransportHTTPConfig) error {
	endpoint := config.Host
	if config.Endpoint != "" {
		endpoint = strings.TrimSuffix(endpoint, "/") + "/" + strings.TrimPrefix(config.Endpoint, "/")
	}

	// Try a simple GET to check connectivity (SOAP endpoints typically accept this)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	if err := t.applyAuth(req, config.Authentication); err != nil {
		return fmt.Errorf("failed to apply authentication: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// BuildSOAPRequest creates a complete SOAP request envelope for debugging/testing.
func (t *SOAPTransport) BuildSOAPRequest(config models.TransportHTTPConfig, request *Request) ([]byte, error) {
	soapBody, err := t.buildSOAPBody(config, request)
	if err != nil {
		return nil, err
	}

	// Build proper SOAP envelope with namespace prefix
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	buf.WriteString(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">`)
	buf.WriteString("\n  <soap:Body>\n    ")
	buf.Write(soapBody)
	buf.WriteString("\n  </soap:Body>\n")
	buf.WriteString("</soap:Envelope>")

	return buf.Bytes(), nil
}
