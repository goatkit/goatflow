// Tiptap Bundle - Exports all required Tiptap modules for bundling
// This gets compiled to tiptap.min.js for airgapped environments

import { Editor } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import Placeholder from '@tiptap/extension-placeholder';
import Link from '@tiptap/extension-link';
import Table from '@tiptap/extension-table';
import TableRow from '@tiptap/extension-table-row';
import TableCell from '@tiptap/extension-table-cell';
import TableHeader from '@tiptap/extension-table-header';

// Export everything as a global object
window.Tiptap = {
    Editor,
    StarterKit,
    Placeholder,
    Link,
    Table,
    TableRow,
    TableCell,
    TableHeader
};

// For compatibility
window.TiptapEditor = Editor;
window.TiptapStarterKit = { StarterKit };
window.TiptapExtensionPlaceholder = { Placeholder };
window.TiptapExtensionLink = { Link };
window.TiptapExtensionTable = { Table };
window.TiptapExtensionTableRow = { TableRow };
window.TiptapExtensionTableCell = { TableCell };
window.TiptapExtensionTableHeader = { TableHeader };