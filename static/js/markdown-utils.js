// Markdown utilities for Tiptap
// Simple HTML ↔ Markdown conversion

// Convert HTML to Markdown
export function htmlToMarkdown(html) {
    if (!html) return '';

    // Create a temporary element to parse HTML
    const temp = document.createElement('div');
    temp.innerHTML = html;

    let markdown = convertElementToMarkdown(temp);
    // Normalize: collapse 3+ consecutive newlines to 2
    markdown = markdown.replace(/\n{3,}/g, '\n\n');
    return markdown;
}

// Convert Markdown to HTML
export function markdownToHTML(markdown) {
    if (!markdown) return '';

    // Simple markdown parser - handles basic formatting
    let html = markdown;

    // Code blocks (must come first) - stash as placeholders so the inline
    // conversion passes below cannot corrupt fence content.
    const codeBlocks = [];
    html = html.replace(/```([\s\S]*?)```/g, (match, code) => {
        codeBlocks.push('<pre><code>' + code + '</code></pre>');
        return '\u0000' + (codeBlocks.length - 1) + '\u0000';
    });

    // GFM tables - must run before the inline passes since cells are
    // delimited by '|'. Escaped pipes ('\\|') inside cells are preserved.
    html = html.replace(/(?:^[ \t]*\|[^\n]*(?:\n|$))+/gm, (block) => {
        const lines = block.trim().split('\n');
        if (lines.length < 2) return block;
        const cut = (line) => {
            line = line.replace(/\\\|/g, '\u0001').trim();
            if (line.charAt(0) === '|') line = line.slice(1);
            if (line.charAt(line.length - 1) === '|') line = line.slice(0, -1);
            return line.split('|').map((cell) => cell.trim().replace(/\u0001/g, '\\|'));
        };
        const separator = cut(lines[1]);
        if (!separator.length || !separator.every((cell) => /^:?-+:?$/.test(cell))) {
            return block;
        }
        let out = '<table><thead><tr>'
            + cut(lines[0]).map((cell) => '<th>' + cell + '</th>').join('')
            + '</tr></thead><tbody>';
        for (let i = 2; i < lines.length; i++) {
            out += '<tr>' + cut(lines[i]).map((cell) => '<td>' + cell + '</td>').join('') + '</tr>';
        }
        out += '</tbody></table>';
        return out;
    });

    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Headers
    html = html.replace(/^### (.*$)/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.*$)/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.*$)/gm, '<h1>$1</h1>');
    // Bullet lists with '*' markers - must run before the emphasis passes
    // or a line-leading '*' would be consumed by the italic rule below.
    html = html.replace(/^(\s*)\* +(.*)$/gm, '<li>$2</li>');

    // Bold and italic
    html = html.replace(/\*\*\*(.*?)\*\*\*/g, '<strong><em>$1</em></strong>');
    html = html.replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/\*(.*?)\*/g, '<em>$1</em>');

    // Links
    html = html.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>');

    // Images
    html = html.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1">');

    // Lists
    html = html.replace(/^(\s*)- (.*$)/gm, '<li>$2</li>');
    html = html.replace(/^(\s*)\d+\. (.*$)/gm, '<li>$2</li>');

    // Wrap consecutive list items. Inter-item whitespace is stripped so the
    // line-break pass below cannot turn it into <br> nodes that ProseMirror
    // would parse as empty list items between real ones.
    html = html.replace(/(<li>.*<\/li>\s*)+/g, (m) => '<ul>' + m.replace(/<\/li>\s+/g, '</li>') + '</ul>');

    // Line breaks
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');

    // Restore stashed code blocks
    html = html.replace(/\u0000(\d+)\u0000/g, (match, index) => codeBlocks[Number(index)]);

    // Wrap in paragraph if not already wrapped
    if (!html.match(/^<(h[1-6]|ul|ol|pre|blockquote|table)/)) {
        html = '<p>' + html + '</p>';
    }

    return html;
}

// Convert DOM element to Markdown (simplified)
function convertElementToMarkdown(element) {
    let markdown = '';

    for (const child of element.childNodes) {
        if (child.nodeType === Node.TEXT_NODE) {
            markdown += child.textContent;
        } else if (child.nodeType === Node.ELEMENT_NODE) {
            const tagName = child.tagName.toLowerCase();

            switch (tagName) {
                case 'h1':
                    markdown += '# ' + convertElementToMarkdown(child) + '\n\n';
                    break;
                case 'h2':
                    markdown += '## ' + convertElementToMarkdown(child) + '\n\n';
                    break;
                case 'h3':
                    markdown += '### ' + convertElementToMarkdown(child) + '\n\n';
                    break;
                case 'strong':
                case 'b':
                    markdown += '**' + convertElementToMarkdown(child) + '**';
                    break;
                case 'em':
                case 'i':
                    markdown += '*' + convertElementToMarkdown(child) + '*';
                    break;
                case 'code':
                    if (child.parentElement && child.parentElement.tagName.toLowerCase() === 'pre') {
                        // Code block - handled by parent
                        markdown += convertElementToMarkdown(child);
                    } else {
                        markdown += '`' + convertElementToMarkdown(child) + '`';
                    }
                    break;
                case 'pre':
                    const codeContent = convertElementToMarkdown(child).trim();
                    markdown += '```\n' + codeContent + '\n```\n\n';
                    break;
                case 'a':
                    const href = child.getAttribute('href');
                    markdown += '[' + convertElementToMarkdown(child) + '](' + href + ')';
                    break;
                case 'img':
                    const src = child.getAttribute('src');
                    const alt = child.getAttribute('alt') || '';
                    markdown += '![' + alt + '](' + src + ')';
                    break;
                case 'ul':
                case 'ol':
                    const listItems = child.querySelectorAll('li');
                    listItems.forEach((li, index) => {
                        const prefix = tagName === 'ul' ? '- ' : (index + 1) + '. ';
                        markdown += prefix + convertElementToMarkdown(li) + '\n';
                    });
                    markdown += '\n';
                    break;
                case 'li':
                    markdown += convertElementToMarkdown(child);
                    break;
                case 'br':
                    markdown += '\n';
                    break;
                case 'p':
                    const pContent = convertElementToMarkdown(child);
                    if (pContent.trim()) {
                        markdown += pContent + '\n\n';
                    }
                    break;
                case 'table': {
                    const rows = Array.from(child.querySelectorAll('tr'));
                    if (rows.length) {
                        const rowToMarkdown = (row) => {
                            const cells = Array.from(row.children).map((cell) =>
                                convertElementToMarkdown(cell)
                                    .trim()
                                    .replace(/\n+/g, ' ')
                                    .replace(/\|/g, '\\|')
                            );
                            return '| ' + cells.join(' | ') + ' |';
                        };
                        const head = rows[0];
                        markdown += rowToMarkdown(head) + '\n';
                        markdown += '|' + Array(head.children.length).fill(' --- ').join('|') + '|\n';
                        rows.slice(1).forEach((row) => {
                            markdown += rowToMarkdown(row) + '\n';
                        });
                        markdown += '\n';
                    }
                    break;
                }
                case 'div':
                case 'span':
                    // Check for styling
                    const style = child.getAttribute('style') || '';
                    if (style.includes('text-decoration: underline')) {
                        markdown += '<u>' + convertElementToMarkdown(child) + '</u>';
                    } else {
                        markdown += convertElementToMarkdown(child);
                    }
                    break;
                default:
                    markdown += convertElementToMarkdown(child);
            }
        }
    }

    return markdown.trim();
}
// cache-bust: force frontend-stage rebuild (GFM table support)
