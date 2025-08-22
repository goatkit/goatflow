package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"github.com/russross/blackfriday/v2"
)

// handleAdminRoadmap displays the ROADMAP.md file as HTML
func handleAdminRoadmap(c *gin.Context) {
	// Read the ROADMAP.md file
	content, err := os.ReadFile("ROADMAP.md")
	if err != nil {
		// If file doesn't exist, show error
		pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/roadmap.pongo2", pongo2.Context{
			"Title":       "Development Roadmap",
			"Error":       "Unable to load roadmap file: " + err.Error(),
			"User":        getUserMapForTemplate(c),
			"ActivePage":  "admin",
		})
		return
	}

	// Convert Markdown to HTML
	htmlContent := blackfriday.Run(content, blackfriday.WithExtensions(
		blackfriday.CommonExtensions|
		blackfriday.AutoHeadingIDs|
		blackfriday.Tables|
		blackfriday.FencedCode|
		blackfriday.Strikethrough|
		blackfriday.SpaceHeadings|
		blackfriday.BackslashLineBreak,
	))

	// Add Tailwind classes to the HTML for better styling
	htmlString := string(htmlContent)
	
	// Add classes to various HTML elements for Tailwind styling
	htmlString = strings.ReplaceAll(htmlString, "<h1", `<h1 class="text-3xl font-bold mb-4 text-gray-900 dark:text-white"`)
	htmlString = strings.ReplaceAll(htmlString, "<h2", `<h2 class="text-2xl font-semibold mb-3 mt-6 text-gray-800 dark:text-gray-100"`)
	htmlString = strings.ReplaceAll(htmlString, "<h3", `<h3 class="text-xl font-medium mb-2 mt-4 text-gray-700 dark:text-gray-200"`)
	htmlString = strings.ReplaceAll(htmlString, "<h4", `<h4 class="text-lg font-medium mb-2 mt-3 text-gray-600 dark:text-gray-300"`)
	htmlString = strings.ReplaceAll(htmlString, "<p>", `<p class="mb-4 text-gray-600 dark:text-gray-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<ul>", `<ul class="list-disc list-inside mb-4 space-y-1 text-gray-600 dark:text-gray-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<ol>", `<ol class="list-decimal list-inside mb-4 space-y-1 text-gray-600 dark:text-gray-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<li>", `<li class="ml-4">`)
	htmlString = strings.ReplaceAll(htmlString, "<code>", `<code class="bg-gray-100 dark:bg-gray-800 px-1 py-0.5 rounded text-sm font-mono text-red-600 dark:text-red-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<pre>", `<pre class="bg-gray-100 dark:bg-gray-800 p-4 rounded-lg overflow-x-auto mb-4">`)
	htmlString = strings.ReplaceAll(htmlString, "<blockquote>", `<blockquote class="border-l-4 border-gotrs-primary pl-4 italic my-4 text-gray-600 dark:text-gray-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<table>", `<table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 mb-6">`)
	htmlString = strings.ReplaceAll(htmlString, "<thead>", `<thead class="bg-gray-50 dark:bg-gray-700">`)
	htmlString = strings.ReplaceAll(htmlString, "<tbody>", `<tbody class="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">`)
	htmlString = strings.ReplaceAll(htmlString, "<th>", `<th class="px-6 py-3 text-left text-xs font-medium text-gray-500 dark:text-gray-300 uppercase tracking-wider">`)
	htmlString = strings.ReplaceAll(htmlString, "<td>", `<td class="px-6 py-4 whitespace-nowrap text-sm text-gray-600 dark:text-gray-400">`)
	htmlString = strings.ReplaceAll(htmlString, "<strong>", `<strong class="font-semibold text-gray-700 dark:text-gray-300">`)
	htmlString = strings.ReplaceAll(htmlString, "<em>", `<em class="italic">`)
	
	// Handle checkboxes in the markdown
	htmlString = strings.ReplaceAll(htmlString, `<input type="checkbox" disabled=""`, `<input type="checkbox" disabled class="mr-2"`)
	htmlString = strings.ReplaceAll(htmlString, `<input type="checkbox" checked="" disabled=""`, `<input type="checkbox" checked disabled class="mr-2"`)

	pongo2Renderer.HTML(c, http.StatusOK, "pages/admin/roadmap.pongo2", pongo2.Context{
		"Title":       "Development Roadmap",
		"HTMLContent": htmlString,
		"User":        getUserMapForTemplate(c),
		"ActivePage":  "admin",
	})
}