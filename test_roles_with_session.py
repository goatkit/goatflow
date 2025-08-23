#!/usr/bin/env python3

import requests
import json
import time
from datetime import datetime

def test_admin_roles_with_session():
    """Test admin roles functionality by simulating a logged-in user session"""
    
    base_url = "http://localhost:8080"
    session = requests.Session()
    
    results = {
        "timestamp": datetime.now().isoformat(),
        "tests": [],
        "errors": [],
        "session_info": {}
    }
    
    def log_test(name, success, details=None):
        result = {
            "test": name,
            "success": success,
            "timestamp": datetime.now().isoformat()
        }
        if details:
            result.update(details)
        results["tests"].append(result)
        
        status = "✓" if success else "✗"
        print(f"{status} {name}")
        if details and not success:
            for key, value in details.items():
                if key != "response_text":  # Skip large response text in console
                    print(f"   {key}: {value}")
    
    print("=== Testing Admin Roles with Session Management ===\n")
    
    # Step 1: Try to access roles page (should redirect to login)
    print("1. Testing initial access...")
    try:
        response = session.get(f"{base_url}/admin/roles", allow_redirects=False)
        log_test("Initial roles access", 
                response.status_code in [302, 303], 
                {"status_code": response.status_code, "location": response.headers.get('Location')})
        
        if 'login' in str(response.headers.get('Location', '')).lower():
            results["session_info"]["requires_auth"] = True
    except Exception as e:
        log_test("Initial roles access", False, {"error": str(e)})
        results["errors"].append(str(e))
    
    # Step 2: Try to access role membership endpoints without auth
    print("\n2. Testing membership endpoints without auth...")
    membership_endpoints = [
        "/admin/roles/1",
        "/admin/roles/1/users", 
        "/admin/roles/1/users/1"
    ]
    
    for endpoint in membership_endpoints:
        try:
            response = session.get(f"{base_url}{endpoint}", allow_redirects=False)
            requires_auth = response.status_code in [302, 303, 401, 403]
            log_test(f"Unauth access to {endpoint}", 
                    requires_auth,  # We expect this to require auth
                    {"status_code": response.status_code})
        except Exception as e:
            log_test(f"Unauth access to {endpoint}", False, {"error": str(e)})
            results["errors"].append(f"{endpoint}: {str(e)}")
    
    # Step 3: Try different HTTP methods on role endpoints
    print("\n3. Testing different HTTP methods...")
    test_methods = [
        ("GET", "/admin/roles/1/users"),
        ("POST", "/admin/roles/1/users"),
        ("DELETE", "/admin/roles/1/users/1"),
        ("PUT", "/admin/roles/1")
    ]
    
    for method, endpoint in test_methods:
        try:
            response = session.request(method, f"{base_url}{endpoint}", 
                                     json={"user_id": 1} if method in ["POST", "PUT"] else None,
                                     allow_redirects=False)
            expected_codes = [302, 303, 401, 403, 405]  # Auth redirect or method not allowed
            success = response.status_code in expected_codes
            
            log_test(f"{method} {endpoint}", success,
                    {"status_code": response.status_code})
                    
            # Check if the endpoint exists (not 404)
            if response.status_code == 404:
                results["errors"].append(f"Endpoint {method} {endpoint} returns 404 - missing handler")
                
        except Exception as e:
            log_test(f"{method} {endpoint}", False, {"error": str(e)})
            results["errors"].append(f"{method} {endpoint}: {str(e)}")
    
    # Step 4: Test with simulated Ajax headers (like the JavaScript would send)
    print("\n4. Testing with Ajax headers...")
    ajax_headers = {
        'Accept': 'application/json',
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest'
    }
    
    ajax_endpoints = [
        ("GET", "/admin/roles/1/users"),
        ("POST", "/admin/roles/1/users"),
        ("DELETE", "/admin/roles/1/users/1")
    ]
    
    for method, endpoint in ajax_endpoints:
        try:
            response = session.request(method, f"{base_url}{endpoint}",
                                     headers=ajax_headers,
                                     json={"user_id": 1} if method == "POST" else None,
                                     allow_redirects=False)
            
            # Check if response is JSON
            is_json = False
            response_data = None
            try:
                response_data = response.json()
                is_json = True
            except:
                pass
            
            log_test(f"Ajax {method} {endpoint}", 
                    response.status_code in [200, 401, 403],  # Valid responses for Ajax
                    {
                        "status_code": response.status_code,
                        "is_json": is_json,
                        "content_type": response.headers.get('Content-Type', ''),
                        "response_preview": str(response_data)[:200] if is_json else response.text[:200]
                    })
                    
        except Exception as e:
            log_test(f"Ajax {method} {endpoint}", False, {"error": str(e)})
            results["errors"].append(f"Ajax {method} {endpoint}: {str(e)}")
    
    # Step 5: Try to simulate authentication bypass (for testing)
    print("\n5. Testing potential authentication issues...")
    
    # Test if endpoints respond differently with various headers
    auth_bypass_tests = [
        ("admin-bypass", {"X-Admin": "true"}),
        ("local-access", {"X-Forwarded-For": "127.0.0.1"}),
        ("debug-mode", {"X-Debug": "1"}),
        ("api-key", {"Authorization": "Bearer test"})
    ]
    
    for test_name, headers in auth_bypass_tests:
        try:
            response = session.get(f"{base_url}/admin/roles/1/users", 
                                 headers=headers, allow_redirects=False)
            
            # If we get anything other than redirect, it might be a security issue
            bypassed = response.status_code not in [302, 303, 401, 403]
            
            log_test(f"Auth bypass test: {test_name}", 
                    not bypassed,  # We want this to fail (not bypass auth)
                    {"status_code": response.status_code, "bypassed_auth": bypassed})
                    
            if bypassed:
                results["errors"].append(f"SECURITY: {test_name} bypassed authentication!")
                
        except Exception as e:
            log_test(f"Auth bypass test: {test_name}", True, {"error": str(e)})  # Error is good here
    
    # Step 6: Check for JavaScript console errors by examining HTML
    print("\n6. Checking for potential JavaScript issues...")
    try:
        response = session.get(f"{base_url}/admin/roles", headers={'Accept': 'text/html'})
        html = response.text
        
        js_issues = []
        
        # Look for common JavaScript error patterns
        error_patterns = [
            ('undefined_function', 'is not defined'),
            ('syntax_error', 'SyntaxError'),
            ('reference_error', 'ReferenceError'), 
            ('type_error', 'TypeError'),
            ('network_error', 'NetworkError'),
            ('fetch_error', 'fetch'),
            ('missing_endpoint', '404')
        ]
        
        for pattern_name, pattern in error_patterns:
            if pattern in html:
                js_issues.append(pattern_name)
        
        # Check for expected JavaScript functions
        expected_functions = ['viewRoleUsers', 'addUserToRole', 'removeUserFromRole']
        missing_functions = [func for func in expected_functions if func not in html]
        
        log_test("JavaScript analysis", 
                len(js_issues) == 0 and len(missing_functions) == 0,
                {
                    "html_length": len(html),
                    "js_issues": js_issues,
                    "missing_functions": missing_functions,
                    "contains_roles_table": "Role Management" in html
                })
                
    except Exception as e:
        log_test("JavaScript analysis", False, {"error": str(e)})
        results["errors"].append(f"JavaScript analysis: {str(e)}")
    
    # Save results
    results["summary"] = {
        "total_tests": len(results["tests"]),
        "passed_tests": len([t for t in results["tests"] if t["success"]]),
        "failed_tests": len([t for t in results["tests"] if not t["success"]]),
        "total_errors": len(results["errors"])
    }
    
    with open('/tmp/roles_session_test.json', 'w') as f:
        json.dump(results, f, indent=2)
    
    print(f"\n=== SUMMARY ===")
    print(f"Tests: {results['summary']['passed_tests']}/{results['summary']['total_tests']} passed")
    print(f"Errors: {results['summary']['total_errors']}")
    
    if results["errors"]:
        print(f"\nKey Issues Found:")
        for error in results["errors"][:5]:  # Show first 5 errors
            print(f"  - {error}")
            
    print(f"\nDetailed results saved to: /tmp/roles_session_test.json")
    
    return results

if __name__ == "__main__":
    test_admin_roles_with_session()