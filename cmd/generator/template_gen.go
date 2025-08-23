package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func generatePongoTemplate(config ModuleConfig, outputDir string) error {
	// Build the Pongo2 template string dynamically
	var template strings.Builder

	// Header
	template.WriteString(`{% extends "layouts/base.pongo2" %}
{% import "macros/forms.pongo2" as forms %}
{% import "macros/tables.pongo2" as tables %}
{% import "macros/modals.pongo2" as modals %}

`)

	// Title block
	fmt.Fprintf(&template, "{%% block title %%}%s Management{%% endblock %%}\n\n", config.Module.Plural)

	// Content block
	template.WriteString(`{% block content %}
<div class="container mx-auto px-4 py-6">
    <div class="mb-6">
`)
	fmt.Fprintf(&template, "        <h1 class=\"text-2xl font-semibold text-gray-900 dark:text-white\">%s Management</h1>\n", config.Module.Plural)
	fmt.Fprintf(&template, "        <p class=\"mt-2 text-sm text-gray-600 dark:text-gray-400\">%s</p>\n", config.Module.Description)
	
	template.WriteString(`    </div>

    <!-- Action Bar -->
    <div class="mb-4 flex justify-between items-center">
        <div class="flex space-x-2">
            <button onclick="openCreateModal()" 
                    class="px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors">
                <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/>
                </svg>
`)
	fmt.Fprintf(&template, "                New %s\n", config.Module.Singular)
	template.WriteString(`            </button>
`)

	if config.Features.ImportCSV {
		template.WriteString(`            <button onclick="openImportModal()" 
                    class="px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 transition-colors">
                <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12"/>
                </svg>
                Import CSV
            </button>
`)
	}

	if config.Features.ExportCSV {
		template.WriteString(`            <button onclick="exportToCSV()" 
                    class="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 transition-colors">
                <svg class="w-4 h-4 inline mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/>
                </svg>
                Export CSV
            </button>
`)
	}

	template.WriteString(`        </div>
    </div>

    <!-- Data Table -->
    <div class="bg-white dark:bg-gray-800 shadow rounded-lg">
`)

	if config.Features.Search {
		template.WriteString(`        <div class="p-4 border-b border-gray-200 dark:border-gray-700">
            <div class="flex justify-between items-center">
                <input type="text" 
                       id="searchInput"
                       placeholder="Search..."
                       class="px-3 py-1.5 border border-gray-300 dark:border-gray-600 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-gray-700 dark:text-white text-sm">
            </div>
        </div>
`)
	}

	// Table
	template.WriteString(`        <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
                <thead class="bg-gray-50 dark:bg-gray-700">
                    <tr>
`)

	// Table headers
	for _, field := range config.Fields {
		if field.ShowInList {
			fmt.Fprintf(&template, "                        <th class=\"px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider\">%s</th>\n", field.Label)
		}
	}
	
	template.WriteString(`                        <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">Actions</th>
                    </tr>
                </thead>
                <tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
                    {% for item in items %}
                    <tr class="hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors">
`)

	// Table rows
	for _, field := range config.Fields {
		if field.ShowInList {
			if field.Name == "valid_id" {
				template.WriteString(`                        <td class="px-6 py-4 whitespace-nowrap text-sm">
                            {% if item.valid_id == 1 %}
                            <span class="px-2 py-1 text-xs rounded-full bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200">Valid</span>
                            {% else %}
                            <span class="px-2 py-1 text-xs rounded-full bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200">Invalid</span>
                            {% endif %}
                        </td>
`)
			} else if field.Type == "color" {
				fmt.Fprintf(&template, `                        <td class="px-6 py-4 whitespace-nowrap text-sm">
                            <div class="flex items-center">
                                <div class="w-6 h-6 rounded" style="background-color: {{ item.%s }}"></div>
                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ item.%s }}</span>
                            </div>
                        </td>
`, field.Name, field.Name)
			} else {
				fmt.Fprintf(&template, "                        <td class=\"px-6 py-4 whitespace-nowrap text-sm text-gray-900 dark:text-gray-100\">{{ item.%s }}</td>\n", field.Name)
			}
		}
	}

	// Action buttons
	template.WriteString(`                        <td class="px-6 py-4 whitespace-nowrap text-right text-sm font-medium">
                            <button onclick="editItem({{ item.id }})" class="text-blue-600 hover:text-blue-900 dark:text-blue-400 dark:hover:text-blue-300 mr-2">
                                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/>
                                </svg>
                            </button>
`)

	if config.Features.SoftDelete {
		template.WriteString(`                            {% if item.valid_id == 1 %}
                            <button onclick="toggleStatus({{ item.id }}, 1)" class="text-yellow-600 hover:text-yellow-900 dark:text-yellow-400 dark:hover:text-yellow-300">
                                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/>
                                </svg>
                            </button>
                            {% else %}
                            <button onclick="toggleStatus({{ item.id }}, 2)" class="text-green-600 hover:text-green-900 dark:text-green-400 dark:hover:text-green-300">
                                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                </svg>
                            </button>
                            {% endif %}
`)
	} else {
		template.WriteString(`                            <button onclick="deleteItem({{ item.id }})" class="text-red-600 hover:text-red-900 dark:text-red-400 dark:hover:text-red-300">
                                <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                                </svg>
                            </button>
`)
	}

	template.WriteString(`                        </td>
                    </tr>
                    {% empty %}
                    <tr>
                        <td colspan="100%" class="px-6 py-12 text-center text-gray-500 dark:text-gray-400">
                            No data available
                        </td>
                    </tr>
                    {% endfor %}
                </tbody>
            </table>
        </div>
    </div>
</div>

<!-- Create/Edit Modal -->
`)

	// Modal
	fmt.Fprintf(&template, `{{ modals.form_modal("%s_modal", "", "%s_form") }}
`, config.Module.Name, config.Module.Name)

	// JavaScript
	template.WriteString(`
<script>
`)
	fmt.Fprintf(&template, "const API_BASE = '%s';\n", config.Module.RoutePrefix)
	template.WriteString(`let currentEditId = null;

function openCreateModal() {
    currentEditId = null;
`)
	fmt.Fprintf(&template, "    document.querySelector('#%s_modal h3').textContent = 'Create New %s';\n", config.Module.Name, config.Module.Singular)
	fmt.Fprintf(&template, "    document.getElementById('%s_form').reset();\n", config.Module.Name)
	fmt.Fprintf(&template, "    openModal('%s_modal');\n", config.Module.Name)
	template.WriteString(`}

function editItem(id) {
    currentEditId = id;
`)
	fmt.Fprintf(&template, "    document.querySelector('#%s_modal h3').textContent = 'Edit %s';\n", config.Module.Name, config.Module.Singular)
	template.WriteString(`    
    fetch(` + "`${API_BASE}/${id}`" + `, {
        headers: {
            'X-Requested-With': 'XMLHttpRequest',
            'Accept': 'application/json'
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
`)

	// Populate form fields
	for _, field := range config.Fields {
		if field.ShowInForm {
			fmt.Fprintf(&template, "            document.getElementById('%s').value = data.data.%s || '';\n", field.Name, field.Name)
		}
	}

	fmt.Fprintf(&template, "            openModal('%s_modal');\n", config.Module.Name)
	template.WriteString(`        }
    });
}

// Form submission
`)
	fmt.Fprintf(&template, "document.getElementById('%s_form').addEventListener('submit', function(e) {\n", config.Module.Name)
	template.WriteString(`    e.preventDefault();
    
    const formData = new FormData(this);
    const url = currentEditId ? ` + "`${API_BASE}/${currentEditId}`" + ` : API_BASE;
    const method = currentEditId ? 'PUT' : 'POST';
    
    fetch(url, {
        method: method,
        headers: {
            'X-Requested-With': 'XMLHttpRequest',
            'Content-Type': 'application/x-www-form-urlencoded'
        },
        body: new URLSearchParams(formData)
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
`)
	fmt.Fprintf(&template, "            closeModal('%s_modal');\n", config.Module.Name)
	template.WriteString(`            showToast(data.message, 'success');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast(data.error || 'An error occurred', 'error');
        }
    });
});

// Toast notification
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = ` + "`fixed bottom-4 right-4 px-6 py-3 rounded-lg text-white shadow-lg transition-opacity duration-500 ${type === 'success' ? 'bg-green-600' : type === 'error' ? 'bg-red-600' : 'bg-blue-600'}`" + `;
    toast.textContent = message;
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.style.opacity = '0';
        setTimeout(() => toast.remove(), 500);
    }, 3000);
}
</script>
{% endblock %}
`)

	// Write to file
	templatePath := filepath.Join(outputDir, "templates", "pages", "admin", fmt.Sprintf("%s.pongo2", config.Module.Name))
	os.MkdirAll(filepath.Dir(templatePath), 0755)

	return os.WriteFile(templatePath, []byte(template.String()), 0644)
}