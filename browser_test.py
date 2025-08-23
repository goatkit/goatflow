#!/usr/bin/env python3

import os
import sys
import time
import json
import subprocess
from datetime import datetime

def run_browser_test():
    """
    Use Firefox in headless mode to test the admin roles page
    """
    
    print("Starting browser automation test for Admin Roles page...")
    
    # Create a simple HTML test page that loads the roles page in an iframe
    # and captures JavaScript console errors
    test_html = '''
<!DOCTYPE html>
<html>
<head>
    <title>GOTRS Roles Test</title>
    <script>
        let errors = [];
        let consoleMessages = [];
        
        // Capture console messages
        const originalLog = console.log;
        const originalError = console.error;
        const originalWarn = console.warn;
        
        console.log = function(...args) {
            consoleMessages.push({type: 'log', message: args.join(' ')});
            originalLog.apply(console, args);
        };
        
        console.error = function(...args) {
            const message = args.join(' ');
            errors.push({type: 'error', message: message});
            consoleMessages.push({type: 'error', message: message});
            originalError.apply(console, args);
        };
        
        console.warn = function(...args) {
            consoleMessages.push({type: 'warn', message: args.join(' ')});
            originalWarn.apply(console, args);
        };
        
        // Capture uncaught errors
        window.onerror = function(message, source, lineno, colno, error) {
            errors.push({
                type: 'uncaught_error',
                message: message,
                source: source,
                line: lineno,
                column: colno,
                stack: error ? error.stack : null
            });
        };
        
        // Capture promise rejections
        window.addEventListener('unhandledrejection', function(event) {
            errors.push({
                type: 'unhandled_rejection',
                message: event.reason
            });
        });
        
        function testRolesPage() {
            console.log('Starting roles page test...');
            
            // Try to load the roles page
            fetch('http://localhost:8080/admin/roles')
                .then(response => {
                    console.log('Roles page response status:', response.status);
                    return response.text();
                })
                .then(html => {
                    console.log('Roles page loaded, length:', html.length);
                    
                    // Try to test the membership API endpoints
                    testMembershipEndpoints();
                })
                .catch(error => {
                    console.error('Failed to load roles page:', error);
                    errors.push({type: 'fetch_error', message: error.toString()});
                });
        }
        
        function testMembershipEndpoints() {
            const testEndpoints = [
                '/admin/roles/1/users',
                '/admin/roles/1',
                '/admin/roles/create'
            ];
            
            testEndpoints.forEach((endpoint, index) => {
                setTimeout(() => {
                    console.log('Testing endpoint:', endpoint);
                    
                    fetch('http://localhost:8080' + endpoint, {
                        method: 'GET',
                        headers: {
                            'Accept': 'application/json'
                        }
                    })
                    .then(response => {
                        console.log(`Endpoint ${endpoint}: ${response.status} ${response.statusText}`);
                        return response.text();
                    })
                    .then(text => {
                        console.log(`Response for ${endpoint} (first 200 chars):`, text.substring(0, 200));
                        
                        if (index === testEndpoints.length - 1) {
                            // Last endpoint tested, save results
                            saveResults();
                        }
                    })
                    .catch(error => {
                        console.error(`Error testing ${endpoint}:`, error);
                        errors.push({type: 'endpoint_error', endpoint: endpoint, message: error.toString()});
                        
                        if (index === testEndpoints.length - 1) {
                            saveResults();
                        }
                    });
                }, index * 1000);
            });
        }
        
        function saveResults() {
            const results = {
                timestamp: new Date().toISOString(),
                errors: errors,
                consoleMessages: consoleMessages,
                summary: {
                    totalErrors: errors.length,
                    totalConsoleMessages: consoleMessages.length,
                    hasErrors: errors.length > 0
                }
            };
            
            console.log('\\n=== FINAL RESULTS ===');
            console.log('Errors found:', errors.length);
            console.log('Console messages:', consoleMessages.length);
            
            // Try to save results to a global variable that can be accessed by the parent
            window.testResults = results;
            
            // Also log the full results
            console.log('Full results:', JSON.stringify(results, null, 2));
        }
        
        // Start test when page loads
        window.onload = function() {
            setTimeout(testRolesPage, 1000);
        };
    </script>
</head>
<body>
    <h1>GOTRS Admin Roles Test</h1>
    <p>Check the browser console for test results...</p>
    <div id="results"></div>
</body>
</html>
'''
    
    # Write test HTML to temp file
    test_file = '/tmp/roles_test.html'
    with open(test_file, 'w') as f:
        f.write(test_html)
    
    print(f"Created test HTML: {test_file}")
    
    # Try to run with firefox if available
    try:
        # Run Firefox in headless mode and dump console output
        cmd = [
            'timeout', '30',  # Max 30 seconds
            'firefox', 
            '--headless',
            '--screenshot=/tmp/roles_screenshot.png',
            f'file://{test_file}'
        ]
        
        print("Running Firefox headless test...")
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=35)
        
        print("Firefox stdout:", result.stdout)
        print("Firefox stderr:", result.stderr)
        print("Firefox return code:", result.returncode)
        
        if os.path.exists('/tmp/roles_screenshot.png'):
            print("Screenshot saved to: /tmp/roles_screenshot.png")
        
    except FileNotFoundError:
        print("Firefox not found, trying alternative approaches...")
        
    except subprocess.TimeoutExpired:
        print("Firefox test timed out")
    
    # Alternative: Use curl to test the actual endpoints directly
    print("\\nTesting endpoints with curl...")
    
    endpoints_to_test = [
        '/admin/roles',
        '/admin/roles/1',
        '/admin/roles/1/users', 
        '/admin/roles/create'
    ]
    
    results = {
        'timestamp': datetime.now().isoformat(),
        'endpoint_tests': [],
        'errors': []
    }
    
    for endpoint in endpoints_to_test:
        print(f"\\nTesting {endpoint}...")
        try:
            # Test with curl
            curl_cmd = [
                'curl', '-s', '-w', 
                '\\nHTTP_CODE:%{http_code}\\nRESPONSE_TIME:%{time_total}\\n',
                '-H', 'Accept: application/json',
                f'http://localhost:8080{endpoint}'
            ]
            
            result = subprocess.run(curl_cmd, capture_output=True, text=True, timeout=10)
            
            lines = result.stdout.split('\\n')
            response_body = '\\n'.join(lines[:-3])  # Everything except last 3 lines
            http_code = lines[-3].replace('HTTP_CODE:', '') if len(lines) >= 3 else 'unknown'
            response_time = lines[-2].replace('RESPONSE_TIME:', '') if len(lines) >= 2 else 'unknown'
            
            test_result = {
                'endpoint': endpoint,
                'http_code': http_code,
                'response_time': response_time,
                'response_body_preview': response_body[:300],
                'success': http_code.startswith('2') or http_code in ['302', '303']
            }
            
            results['endpoint_tests'].append(test_result)
            
            print(f"  HTTP {http_code} in {response_time}s")
            if not test_result['success']:
                print(f"  Response: {response_body[:200]}...")
            
        except subprocess.TimeoutExpired:
            print(f"  Timeout testing {endpoint}")
            results['errors'].append(f"Timeout testing {endpoint}")
        except Exception as e:
            print(f"  Error testing {endpoint}: {e}")
            results['errors'].append(f"Error testing {endpoint}: {e}")
    
    # Try to check browser console for JavaScript errors on the actual page
    print("\\nTesting actual page load with curl...")
    try:
        result = subprocess.run(['curl', '-s', 'http://localhost:8080/admin/roles'], 
                              capture_output=True, text=True, timeout=10)
        
        html_content = result.stdout
        
        # Look for potential JavaScript errors in the HTML
        js_issues = []
        
        if 'onerror' in html_content or 'error' in html_content.lower():
            js_issues.append("HTML contains error-related content")
        
        if 'function' in html_content and 'fetch' in html_content:
            js_issues.append("Page contains JavaScript with fetch calls")
        
        if len(html_content) < 100:
            js_issues.append("HTML response is very short, might be an error page")
            
        results['html_analysis'] = {
            'length': len(html_content),
            'potential_issues': js_issues,
            'contains_roles_content': 'Role Management' in html_content,
            'contains_membership_functions': 'viewRoleUsers' in html_content
        }
        
        print(f"HTML length: {len(html_content)}")
        print(f"Contains role content: {'Role Management' in html_content}")
        print(f"Contains membership functions: {'viewRoleUsers' in html_content}")
        
    except Exception as e:
        print(f"Error analyzing HTML: {e}")
        results['errors'].append(f"Error analyzing HTML: {e}")
    
    # Save results
    results_file = '/tmp/roles_browser_test.json'
    with open(results_file, 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"\\n=== SUMMARY ===")
    print(f"Endpoint tests: {len(results['endpoint_tests'])}")
    print(f"Errors: {len(results['errors'])}")
    print(f"Results saved to: {results_file}")
    
    # Print key findings
    failing_endpoints = [t for t in results['endpoint_tests'] if not t['success']]
    if failing_endpoints:
        print(f"\\nFailing endpoints: {len(failing_endpoints)}")
        for test in failing_endpoints:
            print(f"  {test['endpoint']}: HTTP {test['http_code']}")
    
    return results

if __name__ == "__main__":
    run_browser_test()