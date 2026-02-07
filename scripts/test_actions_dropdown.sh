#!/bin/bash

# Simple test to verify Actions dropdown is present in ticket detail page
echo "🔍 Checking if Actions dropdown is present in ticket_detail.pongo2..."

# Check if the Actions dropdown HTML is in the template
if grep -q "Actions Dropdown" /home/nigel/git/goatkit/goatflow/templates/pages/ticket_detail.pongo2; then
    echo "✅ Actions dropdown comment found in template"
else
    echo "❌ Actions dropdown comment NOT found in template"
    exit 1
fi

# Check for the button
if grep -q 'button onclick="toggleDropdown' /home/nigel/git/goatkit/goatflow/templates/pages/ticket_detail.pongo2; then
    echo "✅ Actions button found in template"
else
    echo "❌ Actions button NOT found in template"
    exit 1
fi

# Check for the dropdown menu
if grep -q 'id="actionsDropdown"' /home/nigel/git/goatkit/goatflow/templates/pages/ticket_detail.pongo2; then
    echo "✅ Actions dropdown menu found in template"
else
    echo "❌ Actions dropdown menu NOT found in template"
    exit 1
fi

# Check for Move to Queue option
if grep -q 'Move to Queue' /home/nigel/git/goatkit/goatflow/templates/pages/ticket_detail.pongo2; then
    echo "✅ Move to Queue option found in template"
else
    echo "❌ Move to Queue option NOT found in template"
    exit 1
fi

# Check if ticket-zoom.js is included
if grep -q 'ticket-zoom.js' /home/nigel/git/goatkit/goatflow/templates/pages/ticket_detail.pongo2; then
    echo "✅ ticket-zoom.js script included in template"
else
    echo "❌ ticket-zoom.js script NOT included in template"
    exit 1
fi

# Check if moveQueue function exists in ticket-zoom.js
if grep -q 'function moveQueue' /home/nigel/git/goatkit/goatflow/static/js/ticket-zoom.js; then
    echo "✅ moveQueue function found in ticket-zoom.js"
else
    echo "❌ moveQueue function NOT found in ticket-zoom.js"
    exit 1
fi

echo ""
echo "🎉 All Actions dropdown components are present!"
echo ""
echo "If you're still not seeing the Actions dropdown in the browser:"
echo "1. Hard refresh the page (Ctrl+F5)"
echo "2. Check browser developer tools for JavaScript errors"
echo "3. Verify you're logged in as an admin/agent user"
echo "4. Check if the page is loading the correct template"