#!/usr/bin/env node

/**
 * Generate API documentation from OpenAPI specification
 * Creates both static HTML documentation and markdown files
 */

const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

const OPENAPI_FILE = path.join(__dirname, '../api/openapi.yaml');
const DOCS_DIR = path.join(__dirname, '../docs/api');
const HTML_OUTPUT = path.join(DOCS_DIR, 'index.html');
const MD_OUTPUT = path.join(DOCS_DIR, 'README.md');

function generateHTMLDocs(spec) {
  const html = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GOTRS API Documentation</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
    <style>
        .swagger-ui .topbar { display: none; }
        .swagger-ui .info { margin: 20px 0; }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js"></script>
    <script>
        const spec = ${JSON.stringify(spec, null, 2)};
        
        SwaggerUIBundle({
            url: '',
            spec: spec,
            dom_id: '#swagger-ui',
            deepLinking: true,
            presets: [
                SwaggerUIBundle.presets.apis,
                SwaggerUIBundle.presets.standalone
            ],
            plugins: [
                SwaggerUIBundle.plugins.DownloadUrl
            ],
            layout: "StandaloneLayout",
            tryItOutEnabled: true,
            requestInterceptor: (req) => {
                // Add any default headers or modifications here
                return req;
            }
        });
    </script>
</body>
</html>`;

  return html;
}

function generateMarkdownDocs(spec) {
  let md = `# ${spec.info.title}\n\n`;
  md += `${spec.info.description}\n\n`;
  md += `**Version:** ${spec.info.version}\n\n`;

  if (spec.info.contact) {
    md += `**Contact:** [${spec.info.contact.name}](${spec.info.contact.url})\n\n`;
  }

  if (spec.info.license) {
    md += `**License:** [${spec.info.license.name}](${spec.info.license.url})\n\n`;
  }

  // Servers
  if (spec.servers && spec.servers.length > 0) {
    md += `## Servers\n\n`;
    spec.servers.forEach(server => {
      md += `- **${server.description}**: \`${server.url}\`\n`;
    });
    md += `\n`;
  }

  // Security
  if (spec.components && spec.components.securitySchemes) {
    md += `## Authentication\n\n`;
    Object.entries(spec.components.securitySchemes).forEach(([name, scheme]) => {
      md += `### ${name}\n\n`;
      md += `- **Type:** ${scheme.type}\n`;
      if (scheme.scheme) md += `- **Scheme:** ${scheme.scheme}\n`;
      if (scheme.bearerFormat) md += `- **Format:** ${scheme.bearerFormat}\n`;
      md += `\n`;
    });
  }

  // Endpoints
  md += `## Endpoints\n\n`;

  // Group endpoints by tags
  const endpointsByTag = {};
  
  Object.entries(spec.paths || {}).forEach(([path, pathItem]) => {
    Object.entries(pathItem).forEach(([method, operation]) => {
      const tag = operation.tags ? operation.tags[0] : 'Default';
      if (!endpointsByTag[tag]) {
        endpointsByTag[tag] = [];
      }
      endpointsByTag[tag].push({
        method: method.toUpperCase(),
        path,
        operation
      });
    });
  });

  Object.entries(endpointsByTag).forEach(([tag, endpoints]) => {
    md += `### ${tag}\n\n`;
    
    endpoints.forEach(({ method, path, operation }) => {
      md += `#### ${method} ${path}\n\n`;
      
      if (operation.summary) {
        md += `**${operation.summary}**\n\n`;
      }
      
      if (operation.description) {
        md += `${operation.description}\n\n`;
      }

      // Parameters
      if (operation.parameters && operation.parameters.length > 0) {
        md += `**Parameters:**\n\n`;
        operation.parameters.forEach(param => {
          md += `- \`${param.name}\` (${param.in})`;
          if (param.required) md += ` **required**`;
          if (param.description) md += ` - ${param.description}`;
          md += `\n`;
        });
        md += `\n`;
      }

      // Request body
      if (operation.requestBody) {
        md += `**Request Body:**\n\n`;
        if (operation.requestBody.description) {
          md += `${operation.requestBody.description}\n\n`;
        }
        md += `Content-Type: \`application/json\`\n\n`;
      }

      // Responses
      if (operation.responses) {
        md += `**Responses:**\n\n`;
        Object.entries(operation.responses).forEach(([status, response]) => {
          md += `- **${status}**: ${response.description}\n`;
        });
        md += `\n`;
      }

      md += `---\n\n`;
    });
  });

  // Schemas
  if (spec.components && spec.components.schemas) {
    md += `## Data Models\n\n`;
    
    Object.entries(spec.components.schemas).forEach(([name, schema]) => {
      md += `### ${name}\n\n`;
      
      if (schema.description) {
        md += `${schema.description}\n\n`;
      }

      if (schema.type === 'object' && schema.properties) {
        md += `| Field | Type | Required | Description |\n`;
        md += `|-------|------|----------|-------------|\n`;
        
        Object.entries(schema.properties).forEach(([field, prop]) => {
          const required = schema.required && schema.required.includes(field) ? '✓' : '';
          const type = prop.type || 'object';
          const description = prop.description || '';
          md += `| ${field} | ${type} | ${required} | ${description} |\n`;
        });
        md += `\n`;
      }
    });
  }

  return md;
}

function generateDocs() {
  try {
    console.log('📖 Generating API documentation...');

    // Read OpenAPI spec
    const specContent = fs.readFileSync(OPENAPI_FILE, 'utf8');
    const spec = yaml.load(specContent);

    // Ensure docs directory exists
    if (!fs.existsSync(DOCS_DIR)) {
      fs.mkdirSync(DOCS_DIR, { recursive: true });
    }

    // Generate HTML documentation
    const html = generateHTMLDocs(spec);
    fs.writeFileSync(HTML_OUTPUT, html);
    console.log(`✅ Generated HTML docs: ${HTML_OUTPUT}`);

    // Generate Markdown documentation
    const markdown = generateMarkdownDocs(spec);
    fs.writeFileSync(MD_OUTPUT, markdown);
    console.log(`✅ Generated Markdown docs: ${MD_OUTPUT}`);

    // Generate contract summary
    const pathCount = Object.keys(spec.paths || {}).length;
    const schemaCount = Object.keys(spec.components?.schemas || {}).length;
    
    console.log(`📊 Documentation summary:`);
    console.log(`   - ${pathCount} API endpoints`);
    console.log(`   - ${schemaCount} data models`);
    console.log(`   - Interactive HTML documentation available`);
    console.log(`   - Markdown documentation for version control`);

    // Create a simple README for the docs
    const docsReadme = `# GOTRS API Documentation

This directory contains the auto-generated API documentation for GOTRS.

## Files

- \`index.html\` - Interactive Swagger UI documentation
- \`README.md\` - Markdown version of the API documentation

## Viewing Documentation

### Interactive Documentation
Open \`index.html\` in your browser for the full interactive Swagger UI experience.

### Local Development
You can also serve the documentation locally:

\`\`\`bash
# From the project root
npm run serve-docs
\`\`\`

## Regenerating Documentation

Documentation is automatically regenerated from the OpenAPI specification:

\`\`\`bash
# Generate documentation
npm run generate-docs

# Generate both types and documentation
npm run generate-types
npm run generate-docs
\`\`\`

## Contract Testing

This documentation is generated from the same OpenAPI specification used for:
- TypeScript type generation
- Contract validation in CI/CD
- Pact consumer/provider testing

Last generated: ${new Date().toISOString()}
`;

    fs.writeFileSync(path.join(DOCS_DIR, '../CONTRACT-TESTING.md'), docsReadme);
    console.log(`✅ Generated contract testing guide`);

  } catch (error) {
    console.error('❌ Error generating documentation:', error.message);
    process.exit(1);
  }
}

// Run if called directly
if (require.main === module) {
  generateDocs();
}

module.exports = { generateDocs };