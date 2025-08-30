// GOTRS Tiptap Rich Text Editor
// MIT Licensed editor for composing and viewing articles

let editors = {};

// Wait for DOM and Tiptap to be ready
document.addEventListener('DOMContentLoaded', function() {
    // Make initTiptapEditor available globally
    window.initTiptapEditor = initTiptapEditor;
});

function initTiptapEditor(elementId, options = {}) {
    const container = document.getElementById(elementId);
    if (!container) return null;
    
    // Default options
    const config = {
        mode: options.mode || 'edit', // 'edit' or 'view'
        placeholder: options.placeholder || 'Write your message here...',
        content: options.content || '',
        onUpdate: options.onUpdate || null
    };
    
    // Build editor div structure
    const editorHtml = `
        <div class="tiptap-editor ${config.mode === 'view' ? 'readonly' : ''}" data-editor-id="${elementId}">
            ${config.mode === 'edit' ? `
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
                
                <!-- Lists -->
                <div class="flex gap-1 border-r border-gray-200 dark:border-gray-700 pr-2">
                    <button type="button" data-action="bulletList" class="toolbar-btn" title="Bullet List">
                        <i class="fas fa-list-ul"></i>
                    </button>
                    <button type="button" data-action="orderedList" class="toolbar-btn" title="Numbered List">
                        <i class="fas fa-list-ol"></i>
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
            ` : ''}
            <div class="tiptap-content prose prose-sm dark:prose-invert max-w-none p-4 min-h-[200px] focus:outline-none ${config.mode === 'edit' ? 'border border-gray-300 dark:border-gray-600 rounded-b-lg' : ''}"></div>
        </div>
    `;
    
    container.innerHTML = editorHtml;
    
    // Initialize Tiptap editor (using bundled Tiptap)
    // Wait for Tiptap to be loaded
    if (typeof window.Tiptap === 'undefined') {
        console.error('Tiptap not loaded yet');
        return null;
    }
    
    const { Editor, StarterKit, Placeholder, Link, Table, TableRow, TableCell, TableHeader } = window.Tiptap;
    
    const editor = new Editor({
        element: container.querySelector('.tiptap-content'),
        extensions: [
            StarterKit.configure({
                heading: {
                    levels: [1, 2, 3]
                }
            }),
            Placeholder.configure({
                placeholder: config.placeholder
            }),
            Link.configure({
                openOnClick: config.mode === 'view'
            }),
            Table.configure({
                resizable: true
            }),
            TableRow,
            TableCell,
            TableHeader
        ],
        content: config.content,
        editable: config.mode === 'edit',
        onUpdate: ({ editor }) => {
            if (config.onUpdate) {
                config.onUpdate(editor.getHTML());
            }
        }
    });
    
    // Attach toolbar actions for edit mode
    if (config.mode === 'edit') {
        const toolbar = container.querySelector('.tiptap-toolbar');
        toolbar.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-action]');
            if (!btn) return;
            
            e.preventDefault();
            const action = btn.dataset.action;
            
            switch(action) {
                // Text formatting
                case 'bold':
                    editor.chain().focus().toggleBold().run();
                    break;
                case 'italic':
                    editor.chain().focus().toggleItalic().run();
                    break;
                case 'underline':
                    editor.chain().focus().toggleUnderline().run();
                    break;
                case 'strike':
                    editor.chain().focus().toggleStrike().run();
                    break;
                    
                // Headings
                case 'heading1':
                    editor.chain().focus().toggleHeading({ level: 1 }).run();
                    break;
                case 'heading2':
                    editor.chain().focus().toggleHeading({ level: 2 }).run();
                    break;
                case 'heading3':
                    editor.chain().focus().toggleHeading({ level: 3 }).run();
                    break;
                case 'paragraph':
                    editor.chain().focus().setParagraph().run();
                    break;
                    
                // Lists
                case 'bulletList':
                    editor.chain().focus().toggleBulletList().run();
                    break;
                case 'orderedList':
                    editor.chain().focus().toggleOrderedList().run();
                    break;
                    
                // Block elements
                case 'blockquote':
                    editor.chain().focus().toggleBlockquote().run();
                    break;
                case 'codeBlock':
                    editor.chain().focus().toggleCodeBlock().run();
                    break;
                case 'horizontalRule':
                    editor.chain().focus().setHorizontalRule().run();
                    break;
                    
                // Tables
                case 'insertTable':
                    editor.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run();
                    break;
                case 'addColumnBefore':
                    editor.chain().focus().addColumnBefore().run();
                    break;
                case 'addRowAfter':
                    editor.chain().focus().addRowAfter().run();
                    break;
                case 'deleteTable':
                    editor.chain().focus().deleteTable().run();
                    break;
                    
                // Actions
                case 'undo':
                    editor.chain().focus().undo().run();
                    break;
                case 'redo':
                    editor.chain().focus().redo().run();
                    break;
                case 'clearFormat':
                    editor.chain().focus().clearNodes().unsetAllMarks().run();
                    break;
            }
            
            // Update button states
            updateToolbarState(editor, toolbar);
        });
        
        // Update toolbar button states
        editor.on('selectionUpdate', () => {
            updateToolbarState(editor, toolbar);
        });
        
        updateToolbarState(editor, toolbar);
    }
    
    // Store editor instance
    editors[elementId] = editor;
    
    return editor;
}

function updateToolbarState(editor, toolbar) {
    // Update active states for toolbar buttons
    toolbar.querySelectorAll('[data-action]').forEach(btn => {
        const action = btn.dataset.action;
        let isActive = false;
        
        switch(action) {
            case 'bold':
                isActive = editor.isActive('bold');
                break;
            case 'italic':
                isActive = editor.isActive('italic');
                break;
            case 'underline':
                isActive = editor.isActive('underline');
                break;
            case 'strike':
                isActive = editor.isActive('strike');
                break;
            case 'heading1':
                isActive = editor.isActive('heading', { level: 1 });
                break;
            case 'heading2':
                isActive = editor.isActive('heading', { level: 2 });
                break;
            case 'heading3':
                isActive = editor.isActive('heading', { level: 3 });
                break;
            case 'paragraph':
                isActive = editor.isActive('paragraph');
                break;
            case 'bulletList':
                isActive = editor.isActive('bulletList');
                break;
            case 'orderedList':
                isActive = editor.isActive('orderedList');
                break;
            case 'blockquote':
                isActive = editor.isActive('blockquote');
                break;
            case 'codeBlock':
                isActive = editor.isActive('codeBlock');
                break;
        }
        
        if (isActive) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
}

function getEditorContent(elementId) {
    const editor = editors[elementId];
    if (!editor) return '';
    return editor.getHTML();
}

function setEditorContent(elementId, content) {
    const editor = editors[elementId];
    if (!editor) return;
    editor.commands.setContent(content);
}

function destroyEditor(elementId) {
    const editor = editors[elementId];
    if (editor) {
        editor.destroy();
        delete editors[elementId];
    }
}

// Add CSS for toolbar buttons
const style = document.createElement('style');
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
`;
document.head.appendChild(style);

// Export for global use
window.TiptapEditor = {
    init: initTiptapEditor,
    getContent: getEditorContent,
    setContent: setEditorContent,
    destroy: destroyEditor
};