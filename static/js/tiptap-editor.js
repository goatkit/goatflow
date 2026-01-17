// GOTRS Tiptap Rich Text Editor
// Based on Tiptap Simple Editor Template - MIT Licensed

let editors = {};
let tiptapLastKeyboardNavigation = false;

function registerTiptapInstance(elementId, instanceApi) {
    if (!elementId) return;
    editors[elementId] = instanceApi;
}

function getTiptapInstance(elementId) {
    return elementId ? editors[elementId] : undefined;
}

function formatHtmlContent(html) {
    if (typeof html !== "string") return "";
    const trimmed = html.trim();
    if (!trimmed) return "";

    const normalized = trimmed.replace(/>\s+</g, "><");
    const segments = normalized
        .replace(/></g, ">$cutHere<$")
        .split("$cutHere$")
        .map((segment) => segment.trim())
        .filter(Boolean);

    const lines = [];
    let depth = 0;

    segments.forEach((segment) => {
        if (/^<\//.test(segment)) {
            depth = Math.max(depth - 1, 0);
        }

        lines.push(`${"    ".repeat(depth)}${segment}`);

        if (/^<[^!?\/][^>]*[^/]?>$/.test(segment)) {
            depth += 1;
        }
    });

    return lines.join("\n");
}

function isLikelyHtml(content) {
    return typeof content === "string" && /<[^>]+>/.test(content);
}

function normalizeTextareaValue(content) {
    if (!content) return "";
    return isLikelyHtml(content) ? formatHtmlContent(content) : content;
}

function insertTextAtCursor(target, snippet) {
    if (!target) return false;
    const start = target.selectionStart ?? target.value.length;
    const end = target.selectionEnd ?? target.value.length;
    const before = target.value.slice(0, start);
    const after = target.value.slice(end);
    target.value = `${before}${snippet}${after}`;
    const next = start + snippet.length;
    if (typeof target.setSelectionRange === "function") {
        target.setSelectionRange(next, next);
    }
    target.dispatchEvent(new Event("input", { bubbles: true }));
    return true;
}

function insertLiteralIntoMarkdown(elementId, text) {
    const instance = getTiptapInstance(elementId);
    if (!instance || !instance.markdownTextarea) return false;
    return insertTextAtCursor(instance.markdownTextarea, text);
}

function insertLiteralIntoEditor(elementId, text) {
    if (!text) return false;
    const instance = getTiptapInstance(elementId);
    if (!instance) return false;
    if (instance.getMode() === "markdown") {
        return insertLiteralIntoMarkdown(elementId, text);
    }
    if (!instance.editor) return false;
    const payload = { type: "text", text };
    instance.editor.chain().focus().insertContent(payload).run();
    return true;
}

document.addEventListener("keydown", (evt) => {
    if (evt.key === "Tab") {
        tiptapLastKeyboardNavigation = true;
    }
});

["mousedown", "pointerdown", "touchstart"].forEach((type) => {
    document.addEventListener(
        type,
        () => {
            tiptapLastKeyboardNavigation = false;
        },
        { capture: true },
    );
});

// Wait for DOM and Tiptap to be ready
document.addEventListener("DOMContentLoaded", function () {
    // Make initTiptapEditor available globally
    window.initTiptapEditor = initTiptapEditor;
    // Back-compat: templates call TiptapEditor.init(id, opts)
    if (!window.TiptapEditor || typeof window.TiptapEditor !== "object") {
        window.TiptapEditor = {};
    }
    window.TiptapEditor.init = function (elementId, options) {
        return initTiptapEditor(elementId, options);
    };
    window.TiptapEditor.insertLiteral = function (elementId, text) {
        return insertLiteralIntoEditor(elementId, text);
    };
    window.TiptapEditor.insertMarkdownLiteral = function (elementId, text) {
        return insertLiteralIntoMarkdown(elementId, text);
    };

    // Global helper for inserting placeholders from HTML onclick
    window.insertEditorPlaceholder = function (elementId, placeholder) {
        if (typeof TiptapEditor !== "undefined") {
            TiptapEditor.insertText(elementId, placeholder);
        }
    };
});

function initTiptapEditor(elementId, options = {}) {
    console.log("initTiptapEditor called with elementId:", elementId);

    // Check if editor already exists for this element
    const existingInstance = getTiptapInstance(elementId);
    if (existingInstance) {
        console.log("Editor already exists for elementId:", elementId);
        return existingInstance;
    }

    const container = document.getElementById(elementId);
    console.log("Container element:", container);
    if (!container) {
        console.error("Container not found for elementId:", elementId);
        return null;
    }

    // Default options
    const config = {
        mode: options.mode || "edit", // 'edit' or 'view'
        editorMode: options.editorMode || "richtext", // 'richtext' or 'markdown'
        placeholder: options.placeholder || "Write your message here...",
        content: options.content || "",
        onUpdate: options.onUpdate || null,
    };

    // Build editor div structure
    const editorHtml = `
        <div class="tiptap-editor ${config.mode === "view" ? "readonly" : ""}" data-editor-id="${elementId}">
            ${
                config.mode === "edit"
                    ? `
            <div class="tiptap-toolbar border-b border-gray-200 dark:border-gray-700 p-2 flex flex-wrap gap-1">
                <!-- Text formatting -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="bold" class="toolbar-btn" title="Bold (Ctrl+B)">
                        <i class="fas fa-bold"></i>
                    </button>
                    <button type="button" data-action="italic" class="toolbar-btn" title="Italic (Ctrl+I)">
                        <i class="fas fa-italic"></i>
                    </button>
                    <button type="button" data-action="underline" class="toolbar-btn" title="Underline (Ctrl+U)">
                        <i class="fas fa-underline"></i>
                    </button>
                    <button type="button" data-action="strike" class="toolbar-btn" title="Strikethrough">
                        <i class="fas fa-strikethrough"></i>
                    </button>
                </div>

                <!-- Colors and Highlights -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <!-- Text Color Dropdown -->
                    <div class="relative">
                        <button type="button" data-action="textColorMenu" class="toolbar-btn flex items-center gap-1" title="Text Color">
                            <i class="fas fa-palette"></i>
                            <i class="fas fa-chevron-down text-xs"></i>
                        </button>
                        <div class="absolute top-full left-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-md shadow-lg p-2 hidden z-10" data-color-menu="text">
                            <div class="grid grid-cols-6 gap-1 mb-2">
                                <button type="button" data-color="#000000" class="w-6 h-6 rounded border border-gray-300" style="background-color: #000000;" title="Black"></button>
                                <button type="button" data-color="#374151" class="w-6 h-6 rounded border border-gray-300" style="background-color: #374151;" title="Gray"></button>
                                <button type="button" data-color="#DC2626" class="w-6 h-6 rounded border border-gray-300" style="background-color: #DC2626;" title="Red"></button>
                                <button type="button" data-color="#EA580C" class="w-6 h-6 rounded border border-gray-300" style="background-color: #EA580C;" title="Orange"></button>
                                <button type="button" data-color="#CA8A04" class="w-6 h-6 rounded border border-gray-300" style="background-color: #CA8A04;" title="Amber"></button>
                                <button type="button" data-color="#16A34A" class="w-6 h-6 rounded border border-gray-300" style="background-color: #16A34A;" title="Green"></button>
                                <button type="button" data-color="#0891B2" class="w-6 h-6 rounded border border-gray-300" style="background-color: #0891B2;" title="Cyan"></button>
                                <button type="button" data-color="#2563EB" class="w-6 h-6 rounded border border-gray-300" style="background-color: #2563EB;" title="Blue"></button>
                                <button type="button" data-color="#7C3AED" class="w-6 h-6 rounded border border-gray-300" style="background-color: #7C3AED;" title="Violet"></button>
                                <button type="button" data-color="#C026D3" class="w-6 h-6 rounded border border-gray-300" style="background-color: #C026D3;" title="Magenta"></button>
                                <button type="button" data-color="#BE185D" class="w-6 h-6 rounded border border-gray-300" style="background-color: #BE185D;" title="Pink"></button>
                                <button type="button" data-color="#FFFFFF" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FFFFFF;" title="White"></button>
                            </div>
                            <input type="color" data-action="customTextColor" class="w-full h-8 border border-gray-300 rounded" title="Custom Color">
                        </div>
                    </div>

                    <!-- Highlight Color Dropdown -->
                    <div class="relative">
                        <button type="button" data-action="highlightMenu" class="toolbar-btn flex items-center gap-1" title="Highlight Color">
                            <i class="fas fa-highlighter"></i>
                            <i class="fas fa-chevron-down text-xs"></i>
                        </button>
                        <div class="absolute top-full left-0 mt-1 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-600 rounded-md shadow-lg p-2 hidden z-10" data-color-menu="highlight">
                            <div class="grid grid-cols-6 gap-1 mb-2">
                                <button type="button" data-color="#FEF3C7" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FEF3C7;" title="Yellow"></button>
                                <button type="button" data-color="#DBEAFE" class="w-6 h-6 rounded border border-gray-300" style="background-color: #DBEAFE;" title="Blue"></button>
                                <button type="button" data-color="#D1FAE5" class="w-6 h-6 rounded border border-gray-300" style="background-color: #D1FAE5;" title="Green"></button>
                                <button type="button" data-color="#FEE2E2" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FEE2E2;" title="Red"></button>
                                <button type="button" data-color="#F3E8FF" class="w-6 h-6 rounded border border-gray-300" style="background-color: #F3E8FF;" title="Purple"></button>
                                <button type="button" data-color="#FEF7ED" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FEF7ED;" title="Orange"></button>
                                <button type="button" data-color="#F0FDF4" class="w-6 h-6 rounded border border-gray-300" style="background-color: #F0FDF4;" title="Mint"></button>
                                <button type="button" data-color="#ECFDF5" class="w-6 h-6 rounded border border-gray-300" style="background-color: #ECFDF5;" title="Light Green"></button>
                                <button type="button" data-color="#FEFCE8" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FEFCE8;" title="Light Yellow"></button>
                                <button type="button" data-color="#F0F9FF" class="w-6 h-6 rounded border border-gray-300" style="background-color: #F0F9FF;" title="Light Blue"></button>
                                <button type="button" data-color="#FDF4FF" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FDF4FF;" title="Light Purple"></button>
                                <button type="button" data-color="#FAFAFA" class="w-6 h-6 rounded border border-gray-300" style="background-color: #FAFAFA;" title="Gray"></button>
                            </div>
                            <input type="color" data-action="customHighlightColor" class="w-full h-8 border border-gray-300 rounded" title="Custom Color">
                        </div>
                    </div>
                </div>

                <!-- Headings -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="heading1" class="toolbar-btn" title="Heading 1">
                        H1
                    </button>
                    <button type="button" data-action="heading2" class="toolbar-btn" title="Heading 2">
                        H2
                    </button>
                    <button type="button" data-action="heading3" class="toolbar-btn" title="Heading 3">
                        H3
                    </button>
                    <button type="button" data-action="paragraph" class="toolbar-btn" title="Paragraph">
                        <i class="fas fa-paragraph"></i>
                    </button>
                </div>

                <!-- Text Alignment -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="alignLeft" class="toolbar-btn" title="Align Left">
                        <i class="fas fa-align-left"></i>
                    </button>
                    <button type="button" data-action="alignCenter" class="toolbar-btn" title="Align Center">
                        <i class="fas fa-align-center"></i>
                    </button>
                    <button type="button" data-action="alignRight" class="toolbar-btn" title="Align Right">
                        <i class="fas fa-align-right"></i>
                    </button>
                    <button type="button" data-action="alignJustify" class="toolbar-btn" title="Justify">
                        <i class="fas fa-align-justify"></i>
                    </button>
                </div>

                <!-- Lists -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="bulletList" class="toolbar-btn" title="Bullet List">
                        <i class="fas fa-list-ul"></i>
                    </button>
                    <button type="button" data-action="orderedList" class="toolbar-btn" title="Numbered List">
                        <i class="fas fa-list-ol"></i>
                    </button>
                    <button type="button" data-action="taskList" class="toolbar-btn" title="Task List">
                        <i class="fas fa-tasks"></i>
                    </button>
                </div>

                <!-- Block elements -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="blockquote" class="toolbar-btn" title="Blockquote">
                        <i class="fas fa-quote-right"></i>
                    </button>
                    <button type="button" data-action="codeBlock" class="toolbar-btn" title="Code Block">
                        <i class="fas fa-code"></i>
                    </button>
                    <button type="button" data-action="setLink" class="toolbar-btn" title="Insert Link (Ctrl+K)">
                        <i class="fas fa-link"></i>
                    </button>
                    <button type="button" data-action="unsetLink" class="toolbar-btn" title="Remove Link">
                        <i class="fas fa-unlink"></i>
                    </button>
                    <button type="button" data-action="insertImage" class="toolbar-btn" title="Insert Image">
                        <i class="fas fa-image"></i>
                    </button>
                    <button type="button" data-action="horizontalRule" class="toolbar-btn" title="Horizontal Rule">
                        <i class="fas fa-minus"></i>
                    </button>
                </div>

                <!-- Table -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="insertTable" class="toolbar-btn" title="Insert Table">
                        <i class="fas fa-table"></i>
                    </button>
                    <button type="button" data-action="addColumnBefore" class="toolbar-btn" title="Add Column Before">
                        <i class="fas fa-plus-square"></i>
                    </button>
                    <button type="button" data-action="addRowAfter" class="toolbar-btn" title="Add Row After">
                        <i class="fas fa-plus"></i>
                    </button>
                    <button type="button" data-action="deleteTable" class="toolbar-btn" title="Delete Table">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>

                <!-- Actions -->
                <div class="flex gap-1">
                    <button type="button" data-action="toggleMode" class="toolbar-btn mode-toggle" title="Toggle Rich Text/Markdown Mode">
                        <i class="fas fa-code"></i>
                        <span class="mode-text">Rich Text</span>
                    </button>
                    <button type="button" data-action="undo" class="toolbar-btn" title="Undo (Ctrl+Z)">
                        <i class="fas fa-undo"></i>
                    </button>
                    <button type="button" data-action="redo" class="toolbar-btn" title="Redo (Ctrl+Y)">
                        <i class="fas fa-redo"></i>
                    </button>
                    <button type="button" data-action="clearFormat" class="toolbar-btn" title="Clear Formatting">
                        <i class="fas fa-remove-format"></i>
                    </button>
                </div>
            </div>
            `
                    : ""
            }
            <div class="tiptap-content prose prose-sm dark:prose-invert max-w-none p-4 min-h-[200px] focus:outline-none ${config.mode === "edit" ? "border border-gray-300 dark:border-gray-600 rounded-b-lg" : ""}"></div>
        </div>
    `;

    container.innerHTML = editorHtml;
    const toolbar = container.querySelector(".tiptap-toolbar");
    const contentElement = container.querySelector(".tiptap-content");
    if (contentElement) {
        contentElement.setAttribute("tabindex", "0");
        contentElement.style.whiteSpace = "pre-wrap";
    }

    // Initialize Tiptap editor (using bundled Tiptap)
    if (typeof window.Tiptap === "undefined") {
        console.error("Tiptap not loaded");
        return null;
    }

    // Initialize editor based on mode
    let editor = null;
    let markdownTextarea = null;
    let currentMode = config.editorMode;
    const instanceApi = {
        getMode: () => currentMode,
        get editor() {
            return editor;
        },
        set editor(value) {
            editor = value;
        },
        get markdownTextarea() {
            return markdownTextarea;
        },
        set markdownTextarea(value) {
            markdownTextarea = value;
        },
    };
    registerTiptapInstance(elementId, instanceApi);

    // Update toolbar visibility and mode button text
    function updateToolbarVisibility() {
        const toolbar = container.querySelector(".tiptap-toolbar");
        const modeToggle = container.querySelector(".mode-toggle .mode-text");

        // Get all toolbar sections
        const toolbarSections = toolbar.querySelectorAll(".flex.gap-1");
        const actionsSection = toolbar.querySelector(".flex.gap-1:last-child");

        if (currentMode === "markdown") {
            // Hide all sections except the last one (Actions) which contains mode toggle, undo, redo, clear format
            toolbarSections.forEach((section, index) => {
                if (index < toolbarSections.length - 1) {
                    section.style.display = "none";
                } else {
                    section.style.display = "flex";
                }
            });
            if (modeToggle) modeToggle.textContent = "Markdown";
        } else {
            // Show all sections in rich text mode
            toolbarSections.forEach((section) => {
                section.style.display = "flex";
            });
            if (modeToggle) modeToggle.textContent = "Rich Text";
        }
    }

    // Convert content between formats
    function convertContent(content, fromMode, toMode) {
        if (fromMode === toMode) return content;

        if (fromMode === "richtext" && toMode === "markdown") {
            if (window.Tiptap.htmlToMarkdown) {
                return window.Tiptap.htmlToMarkdown(content);
            }
            return normalizeTextareaValue(content);
        } else if (fromMode === "markdown" && toMode === "richtext") {
            return window.Tiptap.markdownToHTML
                ? window.Tiptap.markdownToHTML(content)
                : content;
        }

        return content;
    }

    // Initialize rich text editor
    function initRichTextEditor(content) {
        if (editor) return;

        const {
            Editor,
            StarterKit,
            Placeholder,
            TextAlign,
            TextStyle,
            Color,
            Highlight,
            Underline,
            TaskList,
            TaskItem,
            Table,
            TableRow,
            TableCell,
            TableHeader,
            Image,
        } = window.Tiptap;

        const contentElement = container.querySelector(".tiptap-content");
        instanceApi.editor = new Editor({
            element: contentElement,
            extensions: [
                StarterKit.configure({
                    heading: {
                        levels: [1, 2, 3],
                    },
                    underline: false, // Disable StarterKit's built-in underline, we add it explicitly below
                }),
                Placeholder.configure({
                    placeholder: config.placeholder,
                }),
                TextAlign.configure({
                    types: ["heading", "paragraph"],
                }),
                TextStyle,
                Color,
                Highlight.configure({
                    multicolor: true,
                }),
                Underline,
                TaskList,
                TaskItem.configure({
                    nested: true,
                }),
                Table.configure({
                    resizable: true,
                }),
                TableRow,
                TableCell,
                TableHeader,
                Image,
            ],
            content: content,
            editable: config.mode === "edit",
            onUpdate: ({ editor: updatedEditor }) => {
                console.log("Rich text editor content updated");
                if (config.onUpdate) {
                    // Preserve HTML content instead of converting to Markdown
                    const html = updatedEditor.getHTML();
                    config.onUpdate(html);
                }
            },
        });
    }

    // Initialize markdown textarea
    function initMarkdownTextarea(content) {
        if (markdownTextarea) return;

        const contentElement = container.querySelector(".tiptap-content");
        const textarea = document.createElement("textarea");
        textarea.className =
            "w-full h-64 p-3 border border-gray-300 dark:border-gray-600 rounded-md font-mono text-sm bg-white dark:bg-gray-800 text-gray-900 dark:text-white";
        textarea.placeholder = config.placeholder;
        textarea.value = content;
        textarea.disabled = config.mode !== "edit";

        contentElement.innerHTML = "";
        contentElement.appendChild(textarea);

        textarea.addEventListener("input", () => {
            console.log("Markdown textarea content updated");
            if (config.onUpdate) {
                config.onUpdate(textarea.value);
            }
        });

        instanceApi.markdownTextarea = textarea;
    }

    // Toggle between modes
    function toggleMode() {
        const newMode = currentMode === "richtext" ? "markdown" : "richtext";
        const currentContent =
            currentMode === "richtext"
                ? editor
                    ? editor.getHTML()
                    : ""
                : markdownTextarea
                  ? markdownTextarea.value
                  : "";

        const convertedContent = convertContent(
            currentContent,
            currentMode,
            newMode,
        );

        // Destroy current editor/textarea
        if (editor) {
            editor.destroy();
            instanceApi.editor = null;
        }
        if (markdownTextarea) {
            markdownTextarea.remove();
            instanceApi.markdownTextarea = null;
        }

        // Switch mode and reinitialize
        currentMode = newMode;
        updateToolbarVisibility();

        if (currentMode === "richtext") {
            initRichTextEditor(convertedContent);
        } else {
            initMarkdownTextarea(convertedContent);
        }
    }

    // Initialize with current mode
    updateToolbarVisibility();
    if (currentMode === "richtext") {
        initRichTextEditor(config.content);
    } else {
        initMarkdownTextarea(config.content);
    }

    // Attach toolbar actions for edit mode
    if (config.mode === "edit") {
        const focusEditorContent = () => {
            if (currentMode === "markdown" && markdownTextarea) {
                markdownTextarea.focus();
                return;
            }
            if (editor) {
                editor.chain().focus().run();
                const dom = editor.view && editor.view.dom;
                if (dom && typeof dom.focus === "function") {
                    dom.focus();
                }
            } else if (contentElement) {
                contentElement.focus();
            }
        };

        if (toolbar && config.mode === "edit") {
            container.addEventListener("focusin", (evt) => {
                if (!tiptapLastKeyboardNavigation) {
                    return;
                }
                if (!toolbar.contains(evt.target)) {
                    return;
                }
                if (
                    evt.relatedTarget &&
                    container.contains(evt.relatedTarget)
                ) {
                    return;
                }
                tiptapLastKeyboardNavigation = false;
                focusEditorContent();
            });
        }

        toolbar.addEventListener("click", (e) => {
            const btn = e.target.closest("[data-action]");
            if (!btn) return;

            e.preventDefault();
            const action = btn.dataset.action;

            switch (action) {
                // Mode toggle
                case "toggleMode":
                    toggleMode();
                    if (currentMode === "markdown" && markdownTextarea) {
                        markdownTextarea.focus();
                    } else {
                        focusEditorContent();
                    }
                    break;

                // Text formatting - only work in rich text mode
                case "bold":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleBold().run();
                    }
                    break;
                case "italic":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleItalic().run();
                    }
                    break;
                case "underline":
                    if (currentMode === "richtext" && editor) {
                        // Trim trailing spaces from selection before applying underline
                        const { from, to } = editor.state.selection;
                        const doc = editor.state.doc;
                        let newTo = to;

                        // Check if selection ends with spaces and adjust
                        for (let i = to - 1; i >= from; i--) {
                            const char = doc.textBetween(i, i + 1);
                            if (
                                char !== " " &&
                                char !== "\t" &&
                                char !== "\n"
                            ) {
                                break;
                            }
                            newTo = i;
                        }

                        // Only adjust selection if we found trailing spaces
                        if (newTo !== to) {
                            editor
                                .chain()
                                .focus()
                                .setTextSelection({ from, to: newTo })
                                .toggleMark("underline")
                                .run();
                        } else {
                            editor
                                .chain()
                                .focus()
                                .toggleMark("underline")
                                .run();
                        }
                    }
                    break;
                case "strike":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleStrike().run();
                    }
                    break;

                // Colors
                case "textColorMenu":
                    e.preventDefault();
                    const textColorMenu = container.querySelector(
                        '[data-color-menu="text"]',
                    );
                    if (textColorMenu) {
                        textColorMenu.classList.toggle("hidden");
                        // Close highlight menu if open
                        const highlightMenu = container.querySelector(
                            '[data-color-menu="highlight"]',
                        );
                        if (
                            highlightMenu &&
                            !highlightMenu.classList.contains("hidden")
                        ) {
                            highlightMenu.classList.add("hidden");
                        }
                    }
                    break;
                case "highlightMenu":
                    e.preventDefault();
                    const highlightMenu = container.querySelector(
                        '[data-color-menu="highlight"]',
                    );
                    if (highlightMenu) {
                        highlightMenu.classList.toggle("hidden");
                        // Close text color menu if open
                        const textColorMenu = container.querySelector(
                            '[data-color-menu="text"]',
                        );
                        if (
                            textColorMenu &&
                            !textColorMenu.classList.contains("hidden")
                        ) {
                            textColorMenu.classList.add("hidden");
                        }
                    }
                    break;
                case "customTextColor":
                    if (currentMode === "richtext" && editor) {
                        const textColor = e.target.value;
                        editor.chain().focus().setColor(textColor).run();
                    }
                    break;
                case "customHighlightColor":
                    if (currentMode === "richtext" && editor) {
                        const highlightColor = e.target.value;
                        editor
                            .chain()
                            .focus()
                            .toggleHighlight({ color: highlightColor })
                            .run();
                    }
                    break;

                // Headings
                case "heading1":
                    if (currentMode === "richtext" && editor) {
                        editor
                            .chain()
                            .focus()
                            .toggleHeading({ level: 1 })
                            .run();
                    }
                    break;
                case "heading2":
                    if (currentMode === "richtext" && editor) {
                        editor
                            .chain()
                            .focus()
                            .toggleHeading({ level: 2 })
                            .run();
                    }
                    break;
                case "heading3":
                    if (currentMode === "richtext" && editor) {
                        editor
                            .chain()
                            .focus()
                            .toggleHeading({ level: 3 })
                            .run();
                    }
                    break;
                case "paragraph":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setParagraph().run();
                    }
                    break;

                // Text alignment
                case "alignLeft":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setTextAlign("left").run();
                    }
                    break;
                case "alignCenter":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setTextAlign("center").run();
                    }
                    break;
                case "alignRight":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setTextAlign("right").run();
                    }
                    break;
                case "alignJustify":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setTextAlign("justify").run();
                    }
                    break;

                // Lists
                case "bulletList":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleBulletList().run();
                    }
                    break;
                case "orderedList":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleOrderedList().run();
                    }
                    break;
                case "taskList":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleTaskList().run();
                    }
                    break;

                // Block elements
                case "blockquote":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleBlockquote().run();
                    }
                    break;
                case "codeBlock":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().toggleCodeBlock().run();
                    }
                    break;
                case "setLink":
                    if (currentMode === "richtext" && editor) {
                        const url = prompt("Enter URL:");
                        if (url) {
                            editor.chain().focus().setLink({ href: url }).run();
                        }
                    }
                    break;
                case "unsetLink":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().unsetLink().run();
                    }
                    break;
                case "insertImage":
                    if (currentMode === "richtext" && editor) {
                        const imageUrl = prompt("Enter image URL:");
                        if (imageUrl) {
                            editor
                                .chain()
                                .focus()
                                .setImage({ src: imageUrl })
                                .run();
                        }
                    }
                    break;
                case "horizontalRule":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().setHorizontalRule().run();
                    }
                    break;

                // Tables
                case "insertTable":
                    if (currentMode === "richtext" && editor) {
                        editor
                            .chain()
                            .focus()
                            .insertTable({
                                rows: 3,
                                cols: 3,
                                withHeaderRow: true,
                            })
                            .run();
                    }
                    break;
                case "addColumnBefore":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().addColumnBefore().run();
                    }
                    break;
                case "addRowAfter":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().addRowAfter().run();
                    }
                    break;
                case "deleteTable":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().deleteTable().run();
                    }
                    break;

                // Actions
                case "undo":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().undo().run();
                    }
                    break;
                case "redo":
                    if (currentMode === "richtext" && editor) {
                        editor.chain().focus().redo().run();
                    }
                    break;
                case "clearFormat":
                    if (currentMode === "richtext" && editor) {
                        editor
                            .chain()
                            .focus()
                            .clearNodes()
                            .unsetAllMarks()
                            .run();
                    }
                    break;
            }

            // Update button states
            updateToolbarState(
                currentMode === "richtext" ? editor : null,
                toolbar,
            );
        });

        // Handle color palette button clicks
        toolbar.addEventListener("click", (e) => {
            const colorBtn = e.target.closest("[data-color]");
            if (!colorBtn) return;

            e.preventDefault();
            const color = colorBtn.dataset.color;
            const menu = colorBtn.closest("[data-color-menu]");

            if (menu && menu.dataset.colorMenu === "text") {
                if (currentMode === "richtext" && editor) {
                    editor.chain().focus().setColor(color).run();
                }
            } else if (menu && menu.dataset.colorMenu === "highlight") {
                if (currentMode === "richtext" && editor) {
                    editor
                        .chain()
                        .focus()
                        .toggleHighlight({ color: color })
                        .run();
                }
            }

            // Close the menu after selection
            menu.classList.add("hidden");
        });

        // Close color menus when clicking outside
        document.addEventListener("click", (e) => {
            if (!container.contains(e.target)) {
                container
                    .querySelectorAll("[data-color-menu]")
                    .forEach((menu) => {
                        menu.classList.add("hidden");
                    });
            }
        });

        // Update toolbar button states
        if (currentMode === "richtext" && editor) {
            editor.on("selectionUpdate", () => {
                updateToolbarState(editor, toolbar);
            });

            updateToolbarState(editor, toolbar);
        }
    }

    Object.assign(instanceApi, {
        getHTML: () => {
            if (currentMode === "richtext" && editor) {
                return editor.getHTML();
            }
            if (currentMode === "markdown" && markdownTextarea) {
                return markdownTextarea.value;
            }
            return "";
        },
        getMarkdown: () => {
            if (currentMode === "markdown" && markdownTextarea) {
                return markdownTextarea.value;
            }
            if (currentMode === "richtext" && editor) {
                return window.Tiptap.htmlToMarkdown
                    ? window.Tiptap.htmlToMarkdown(editor.getHTML())
                    : editor.getHTML();
            }
            return "";
        },
        setContent: (content, mode = null) => {
            const targetMode = mode || currentMode;
            if (targetMode === "richtext") {
                const htmlContent =
                    currentMode === "markdown"
                        ? window.Tiptap.markdownToHTML
                            ? window.Tiptap.markdownToHTML(content)
                            : content
                        : content;
                if (editor) {
                    editor.commands.setContent(htmlContent);
                    // Explicitly trigger onUpdate callback since programmatic setContent
                    // may not always trigger Tiptap's onUpdate event
                    if (config.onUpdate) {
                        config.onUpdate(editor.getHTML());
                    }
                }
            } else if (markdownTextarea) {
                const markdownContent =
                    currentMode === "richtext"
                        ? window.Tiptap.htmlToMarkdown
                            ? window.Tiptap.htmlToMarkdown(content)
                            : content
                        : content;
                markdownTextarea.value = markdownContent;
                // Explicitly trigger onUpdate callback for markdown mode too
                if (config.onUpdate) {
                    config.onUpdate(markdownTextarea.value);
                }
            }
        },
        setMode: (mode) => {
            if (mode !== currentMode) {
                toggleMode();
            }
        },
        destroy: () => {
            if (editor) {
                editor.destroy();
                instanceApi.editor = null;
            }
            if (markdownTextarea) {
                markdownTextarea.remove();
                instanceApi.markdownTextarea = null;
            }
        },
    });

    return instanceApi;
}

function updateToolbarState(editor, toolbar) {
    if (!editor) {
        // In markdown mode, disable most buttons but keep toggleMode enabled
        toolbar.querySelectorAll("[data-action]").forEach((btn) => {
            if (btn.dataset.action === "toggleMode") {
                btn.disabled = false;
                btn.classList.remove("opacity-50", "cursor-not-allowed");
            } else {
                btn.disabled = true;
                btn.classList.remove("active");
                btn.classList.add("opacity-50", "cursor-not-allowed");
            }
        });
        return;
    }

    // Enable buttons and update states for rich text mode
    toolbar.querySelectorAll("[data-action]").forEach((btn) => {
        btn.disabled = false;
        btn.classList.remove("opacity-50", "cursor-not-allowed");
    });

    // Update active states for toolbar buttons
    toolbar.querySelectorAll("[data-action]").forEach((btn) => {
        const action = btn.dataset.action;
        let isActive = false;

        switch (action) {
            case "bold":
                isActive = editor.isActive("bold");
                break;
            case "italic":
                isActive = editor.isActive("italic");
                break;
            case "underline":
                isActive = editor.isActive("underline");
                break;
            case "strike":
                isActive = editor.isActive("strike");
                break;
            case "heading1":
                isActive = editor.isActive("heading", { level: 1 });
                break;
            case "heading2":
                isActive = editor.isActive("heading", { level: 2 });
                break;
            case "heading3":
                isActive = editor.isActive("heading", { level: 3 });
                break;
            case "paragraph":
                isActive = editor.isActive("paragraph");
                break;
            case "alignLeft":
                isActive = editor.isActive({ textAlign: "left" });
                break;
            case "alignCenter":
                isActive = editor.isActive({ textAlign: "center" });
                break;
            case "alignRight":
                isActive = editor.isActive({ textAlign: "right" });
                break;
            case "alignJustify":
                isActive = editor.isActive({ textAlign: "justify" });
                break;
            case "bulletList":
                isActive = editor.isActive("bulletList");
                break;
            case "orderedList":
                isActive = editor.isActive("orderedList");
                break;
            case "taskList":
                isActive = editor.isActive("taskList");
                break;
            case "blockquote":
                isActive = editor.isActive("blockquote");
                break;
            case "codeBlock":
                isActive = editor.isActive("codeBlock");
                break;
            case "setLink":
            case "unsetLink":
                isActive = editor.isActive("link");
                break;
        }

        if (isActive) {
            btn.classList.add("active");
        } else {
            btn.classList.remove("active");
        }
    });
}

function getEditorContent(elementId) {
    const instance = getTiptapInstance(elementId);
    if (!instance) return "";

    // Handle markdown mode - return textarea value directly
    if (
        instance.getMode &&
        instance.getMode() === "markdown" &&
        instance.markdownTextarea
    ) {
        return instance.markdownTextarea.value;
    }

    // Handle rich text mode
    if (!instance.editor) return "";
    return instance.editor.getHTML();
}

function setEditorContent(elementId, content, mode) {
    const instance = getTiptapInstance(elementId);
    if (!instance) return;

    // Use the instance's setContent method which properly triggers onUpdate callback
    if (instance.setContent) {
        instance.setContent(content, mode);
        return;
    }

    // Fallback: Handle markdown mode - set textarea value directly
    if (
        instance.getMode &&
        instance.getMode() === "markdown" &&
        instance.markdownTextarea
    ) {
        instance.markdownTextarea.value = content;
        return;
    }

    // Fallback: Handle rich text mode
    if (instance.editor) {
        instance.editor.commands.setContent(content, mode);
    }
}

function setEditorMode(elementId, mode) {
    const instance = getTiptapInstance(elementId);
    if (!instance || !instance.setMode) return;
    instance.setMode(mode);
}

function insertText(elementId, text) {
    return insertLiteralIntoEditor(elementId, text);
}

function destroyEditor(elementId) {
    const instance = getTiptapInstance(elementId);
    if (!instance) return;
    if (instance.editor) {
        instance.editor.destroy();
    }
    delete editors[elementId];
}

// Add CSS for toolbar buttons
const style = document.createElement("style");
style.textContent = `
    .tiptap-toolbar .toolbar-btn {
        padding: 6px 10px;
        border-radius: 4px;
        background: transparent;
        color: #4B5563;
        transition: all 0.2s;
        font-size: 14px;
        min-width: 28px;
        height: 28px;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .dark .tiptap-toolbar .toolbar-btn {
        color: #D1D5DB;
    }

    .tiptap-toolbar .toolbar-btn:hover {
        background: #F3F4F6;
        color: #1F2937;
    }

    .dark .tiptap-toolbar .toolbar-btn:hover {
        background: #374151;
        color: #F9FAFB;
    }

    .tiptap-toolbar .toolbar-btn.active {
        background: #3B82F6;
        color: white;
    }

    .tiptap-content .ProseMirror {
        min-height: inherit;
        outline: none;
    }

    .tiptap-content .ProseMirror p.is-editor-empty:first-child::before {
        color: #9CA3AF;
        content: attr(data-placeholder);
        float: left;
        height: 0;
        pointer-events: none;
    }

    .tiptap-content table {
        border-collapse: collapse;
        table-layout: fixed;
        width: 100%;
        margin: 0;
        overflow: hidden;
    }

    .tiptap-content td, .tiptap-content th {
        min-width: 1em;
        border: 2px solid #D1D5DB;
        padding: 3px 5px;
        vertical-align: top;
        box-sizing: border-box;
        position: relative;
    }

    .dark .tiptap-content td, .dark .tiptap-content th {
        border-color: #4B5563;
    }

    .tiptap-content th {
        background-color: #F3F4F6;
        font-weight: bold;
    }

    .dark .tiptap-content th {
        background-color: #374151;
    }

    .tiptap-content .selectedCell:after {
        z-index: 2;
        position: absolute;
        content: "";
        left: 0; right: 0; top: 0; bottom: 0;
        background: rgba(200, 200, 255, 0.4);
        pointer-events: none;
    }

    .tiptap-content .column-resize-handle {
        position: absolute;
        right: -2px;
        top: 0;
        bottom: -2px;
        width: 4px;
        background-color: #adf;
        pointer-events: none;
    }

    /* Image styles */
    .tiptap-content img {
        max-width: 100%;
        height: auto;
        border-radius: 4px;
    }

    .tiptap-content img.ProseMirror-selectednode {
        outline: 3px solid #68CEF8;
    }

    /* Task list styles */
    .tiptap-content ul[data-type="taskList"] {
        list-style: none;
        padding: 0;
    }

    .tiptap-content ul[data-type="taskList"] li {
        display: flex;
        align-items: flex-start;
    }

    .tiptap-content ul[data-type="taskList"] li > label {
        display: flex;
        align-items: center;
        margin-right: 0.5rem;
        user-select: none;
    }

    .tiptap-content ul[data-type="taskList"] li > label input[type="checkbox"] {
        margin-right: 0.5rem;
    }

    .tiptap-content ul[data-type="taskList"] li[data-checked="true"] {
        text-decoration: line-through;
        opacity: 0.6;
    }
`;
document.head.appendChild(style);

// Export for global use
window.TiptapEditor = {
    init: initTiptapEditor,
    getContent: getEditorContent,
    setContent: setEditorContent,
    setMode: setEditorMode,
    insertText: insertText,
    destroy: destroyEditor,
};
