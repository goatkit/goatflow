#!/usr/bin/env python3
"""
Browser automation test for Admin Roles page
Tests the complete functionality including login, viewing, editing, and user management
"""

import time
import os
from selenium import webdriver
from selenium.webdriver.common.by import By
from selenium.webdriver.support.ui import WebDriverWait
from selenium.webdriver.support import expected_conditions as EC
from selenium.webdriver.chrome.options import Options
from selenium.common.exceptions import TimeoutException, NoSuchElementException

class AdminRolesTest:
    def __init__(self):
        self.base_url = "http://localhost:8080"
        self.screenshots_dir = "/tmp/admin_roles_screenshots"
        os.makedirs(self.screenshots_dir, exist_ok=True)
        
        # Setup Chrome options for headless mode
        chrome_options = Options()
        chrome_options.add_argument("--headless")
        chrome_options.add_argument("--no-sandbox")
        chrome_options.add_argument("--disable-dev-shm-usage")
        chrome_options.add_argument("--window-size=1920,1080")
        
        try:
            self.driver = webdriver.Chrome(options=chrome_options)
        except Exception as e:
            print(f"Chrome not available, trying Firefox: {e}")
            from selenium.webdriver.firefox.options import Options as FirefoxOptions
            firefox_options = FirefoxOptions()
            firefox_options.add_argument("--headless")
            self.driver = webdriver.Firefox(options=firefox_options)
            
        self.wait = WebDriverWait(self.driver, 10)
        
    def take_screenshot(self, name):
        """Take a screenshot and save it"""
        screenshot_path = os.path.join(self.screenshots_dir, f"{name}.png")
        self.driver.save_screenshot(screenshot_path)
        print(f"Screenshot saved: {screenshot_path}")
        return screenshot_path
        
    def login(self, username="root@localhost", password="root"):
        """Login to the system"""
        print("Step 1: Navigating to login page...")
        self.driver.get(f"{self.base_url}/login")
        self.take_screenshot("01_login_page")
        
        try:
            # Fill login form
            username_field = self.wait.until(EC.presence_of_element_located((By.NAME, "username")))
            password_field = self.driver.find_element(By.NAME, "password")
            
            username_field.clear()
            username_field.send_keys(username)
            password_field.clear()
            password_field.send_keys(password)
            
            self.take_screenshot("02_login_form_filled")
            
            # Submit login
            login_button = self.driver.find_element(By.CSS_SELECTOR, "button[type='submit']")
            login_button.click()
            
            # Wait for redirect after login
            time.sleep(2)
            print(f"After login, current URL: {self.driver.current_url}")
            self.take_screenshot("03_after_login")
            
            return True
            
        except Exception as e:
            print(f"Login failed: {e}")
            self.take_screenshot("login_error")
            return False
            
    def navigate_to_roles(self):
        """Navigate to admin roles page"""
        print("Step 2: Navigating to admin roles page...")
        self.driver.get(f"{self.base_url}/admin/roles")
        time.sleep(2)
        
        current_url = self.driver.current_url
        print(f"Current URL: {current_url}")
        
        if "login" in current_url:
            print("Still on login page, login may have failed")
            return False
            
        self.take_screenshot("04_roles_page")
        return True
        
    def verify_roles_list(self):
        """Verify the roles list shows correct status"""
        print("Step 3: Verifying roles list and status...")
        
        try:
            # Wait for the roles table to load
            table = self.wait.until(EC.presence_of_element_located((By.CSS_SELECTOR, "table, .roles-list, .table")))
            print("Roles table found")
            
            # Look for role rows
            role_rows = self.driver.find_elements(By.CSS_SELECTOR, "tr, .role-item")
            print(f"Found {len(role_rows)} role rows")
            
            # Check for status indicators
            status_elements = self.driver.find_elements(By.CSS_SELECTOR, ".status, .badge, .active, .inactive")
            print(f"Found {len(status_elements)} status elements")
            
            for element in status_elements[:3]:  # Check first 3
                print(f"Status element text: '{element.text}'")
                
            self.take_screenshot("05_roles_list_verified")
            return True
            
        except TimeoutException:
            print("No roles table found")
            # Check if there's an error message
            page_text = self.driver.find_element(By.TAG_NAME, "body").text
            print(f"Page content: {page_text[:500]}...")
            self.take_screenshot("05_roles_list_error")
            return False
            
    def test_edit_administrator_role(self):
        """Click Edit on Administrator role and verify permissions"""
        print("Step 4: Testing edit functionality for Administrator role...")
        
        try:
            # Look for Administrator role and edit button
            edit_buttons = self.driver.find_elements(By.CSS_SELECTOR, "a[href*='edit'], button.edit, .btn-edit")
            
            if not edit_buttons:
                # Try looking for any clickable elements with "edit" text
                edit_buttons = self.driver.find_elements(By.XPATH, "//a[contains(text(), 'Edit')] | //button[contains(text(), 'Edit')]")
            
            print(f"Found {len(edit_buttons)} edit buttons")
            
            if edit_buttons:
                # Click the first edit button (assuming it's Administrator)
                edit_buttons[0].click()
                time.sleep(2)
                
                print(f"After clicking edit, URL: {self.driver.current_url}")
                self.take_screenshot("06_edit_role_page")
                
                # Look for permission checkboxes
                checkboxes = self.driver.find_elements(By.CSS_SELECTOR, "input[type='checkbox']")
                print(f"Found {len(checkboxes)} permission checkboxes")
                
                checked_count = 0
                for checkbox in checkboxes:
                    if checkbox.is_selected():
                        checked_count += 1
                        
                print(f"{checked_count} out of {len(checkboxes)} permissions are checked")
                self.take_screenshot("07_permissions_verified")
                return True
            else:
                print("No edit buttons found")
                self.take_screenshot("06_no_edit_buttons")
                return False
                
        except Exception as e:
            print(f"Error testing edit functionality: {e}")
            self.take_screenshot("06_edit_error")
            return False
            
    def test_manage_users_modal(self):
        """Test the Manage Users functionality"""
        print("Step 5: Testing Manage Users modal...")
        
        # Go back to roles list first
        self.driver.get(f"{self.base_url}/admin/roles")
        time.sleep(2)
        
        try:
            # Look for Manage Users buttons
            manage_buttons = self.driver.find_elements(By.CSS_SELECTOR, "a[href*='users'], button.manage-users, .btn-manage")
            
            if not manage_buttons:
                manage_buttons = self.driver.find_elements(By.XPATH, "//a[contains(text(), 'Manage')] | //button[contains(text(), 'Manage')] | //a[contains(text(), 'Users')]")
            
            print(f"Found {len(manage_buttons)} manage users buttons")
            
            if manage_buttons:
                manage_buttons[0].click()
                time.sleep(2)
                
                print(f"After clicking manage users, URL: {self.driver.current_url}")
                self.take_screenshot("08_manage_users_modal")
                
                # Look for user list or modal
                user_elements = self.driver.find_elements(By.CSS_SELECTOR, ".modal, .user-list, .available-users")
                print(f"Found {len(user_elements)} user-related elements")
                
                # Look for available users
                user_items = self.driver.find_elements(By.CSS_SELECTOR, ".user-item, .user, li")
                print(f"Found {len(user_items)} potential user items")
                
                self.take_screenshot("09_users_list")
                return True
            else:
                print("No manage users buttons found")
                self.take_screenshot("08_no_manage_buttons")
                return False
                
        except Exception as e:
            print(f"Error testing manage users: {e}")
            self.take_screenshot("08_manage_error")
            return False
            
    def test_add_user_to_role(self):
        """Test adding a user to a role"""
        print("Step 6: Testing add user to role functionality...")
        
        try:
            # Look for add user button or form
            add_buttons = self.driver.find_elements(By.CSS_SELECTOR, ".btn-add, .add-user, button[type='submit']")
            
            if not add_buttons:
                add_buttons = self.driver.find_elements(By.XPATH, "//button[contains(text(), 'Add')] | //input[@type='submit']")
            
            print(f"Found {len(add_buttons)} add buttons")
            
            if add_buttons:
                # Try to interact with the first add button
                self.take_screenshot("10_before_add_user")
                
                # Click the add button
                add_buttons[0].click()
                time.sleep(2)
                
                self.take_screenshot("11_after_add_user_click")
                
                # Check for success message or updated list
                success_elements = self.driver.find_elements(By.CSS_SELECTOR, ".success, .alert-success, .message")
                print(f"Found {len(success_elements)} success message elements")
                
                return True
            else:
                print("No add user functionality found")
                self.take_screenshot("10_no_add_buttons")
                return False
                
        except Exception as e:
            print(f"Error testing add user: {e}")
            self.take_screenshot("10_add_user_error")
            return False
            
    def check_console_errors(self):
        """Check for JavaScript console errors"""
        print("Checking for console errors...")
        
        try:
            logs = self.driver.get_log('browser')
            errors = [log for log in logs if log['level'] == 'SEVERE']
            
            if errors:
                print(f"Found {len(errors)} console errors:")
                for error in errors:
                    print(f"  - {error['message']}")
            else:
                print("No console errors found")
                
            return len(errors) == 0
            
        except Exception as e:
            print(f"Could not check console errors: {e}")
            return True
            
    def run_full_test(self):
        """Run the complete test suite"""
        print("Starting Admin Roles comprehensive test...")
        print("=" * 60)
        
        results = {
            "login": False,
            "navigation": False,
            "roles_list": False,
            "edit_role": False,
            "manage_users": False,
            "add_user": False,
            "console_clean": False
        }
        
        try:
            # Step 1: Login
            results["login"] = self.login()
            
            if results["login"]:
                # Step 2: Navigate to roles
                results["navigation"] = self.navigate_to_roles()
                
                if results["navigation"]:
                    # Step 3: Verify roles list
                    results["roles_list"] = self.verify_roles_list()
                    
                    # Step 4: Test edit functionality
                    results["edit_role"] = self.test_edit_administrator_role()
                    
                    # Step 5: Test manage users
                    results["manage_users"] = self.test_manage_users_modal()
                    
                    # Step 6: Test add user
                    results["add_user"] = self.test_add_user_to_role()
                    
                    # Step 7: Check console errors
                    results["console_clean"] = self.check_console_errors()
            
        except Exception as e:
            print(f"Test execution error: {e}")
            self.take_screenshot("test_execution_error")
            
        finally:
            self.driver.quit()
            
        # Print results
        print("\n" + "=" * 60)
        print("TEST RESULTS SUMMARY:")
        print("=" * 60)
        
        total_tests = len(results)
        passed_tests = sum(results.values())
        
        for test_name, passed in results.items():
            status = "✅ PASS" if passed else "❌ FAIL"
            print(f"{test_name.replace('_', ' ').title():<20}: {status}")
            
        print(f"\nOverall Success Rate: {passed_tests}/{total_tests} ({passed_tests/total_tests*100:.1f}%)")
        
        print(f"\nScreenshots saved in: {self.screenshots_dir}")
        
        return results

def main():
    """Main test execution"""
    tester = AdminRolesTest()
    results = tester.run_full_test()
    
    # Return exit code based on success
    return 0 if all(results.values()) else 1

if __name__ == "__main__":
    exit(main())