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

    // Code blocks (must come first)
    html = html.replace(/```([\s\S]*?)```/g, '<pre><code>$1</code></pre>');

    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');

    // Headers
    html = html.replace(/^### (.*$)/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.*$)/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.*$)/gm, '<h1>$1</h1>');

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

    // Wrap consecutive list items
    html = html.replace(/(<li>.*<\/li>\s*)+/g, '<ul>$&</ul>');

    // Line breaks
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');

    // Wrap in paragraph if not already wrapped
    if (!html.match(/^<(h[1-6]|ul|ol|pre|blockquote)/)) {
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