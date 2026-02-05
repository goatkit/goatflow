// Tiptap Bundle - Exports all required Tiptap modules for bundling
// This gets compiled to tiptap.min.js for airgapped environments

import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import { Table } from '@tiptap/extension-table';
import { TableRow } from '@tiptap/extension-table-row';
import { TableCell } from '@tiptap/extension-table-cell';
import { TableHeader } from '@tiptap/extension-table-header';
import { TextAlign } from '@tiptap/extension-text-align';
import { TextStyle } from '@tiptap/extension-text-style';
import { Color } from '@tiptap/extension-color';
import { Highlight } from '@tiptap/extension-highlight';
import Underline from '@tiptap/extension-underline';
import { TaskList } from '@tiptap/extension-task-list';
import { TaskItem } from '@tiptap/extension-task-item';
import Image from '@tiptap/extension-image';
import { generateHTML, generateJSON } from '@tiptap/core';
import { markdownToHTML, htmlToMarkdown } from './markdown-utils.js';

// Export everything as a global object
window.Tiptap = {
    Editor,
    StarterKit,
    Placeholder,
    Table,
    TableRow,
    TableCell,
    TableHeader,
    TextAlign,
    TextStyle,
    Color,
    Highlight,
    Underline,
    TaskList,
    TaskItem,
    Image,
    generateHTML,
    generateJSON,
    markdownToHTML,
    htmlToMarkdown
};

// For compatibility
window.TiptapEditor = Editor;
window.TiptapStarterKit = { StarterKit };
window.TiptapExtensionPlaceholder = { Placeholder };
window.TiptapExtensionTable = { Table };
window.TiptapExtensionTableRow = { TableRow };
window.TiptapExtensionTableCell = { TableCell };
window.TiptapExtensionTableHeader = { TableHeader };
window.TiptapExtensionTextAlign = { TextAlign };
window.TiptapExtensionTextStyle = { TextStyle };
window.TiptapExtensionColor = { Color };
window.TiptapExtensionHighlight = { Highlight };
window.TiptapExtensionUnderline = { Underline };
window.TiptapExtensionTaskList = { TaskList };
window.TiptapExtensionTaskItem = { TaskItem };
window.TiptapExtensionImage = { Image };
window.TiptapExtensionImage = { Image };