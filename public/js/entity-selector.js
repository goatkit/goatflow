/**
 * EntitySelector - Reusable entity selection modal component for GoatKit
 *
 * A first-class GoatKit component for managing many-to-many entity relationships.
 * Provides: debounced search, local member filtering, optimistic UI updates,
 * undo functionality, and keyboard navigation.
 *
 * Usage:
 *   const selector = new EntitySelector({
 *     modalId: 'roleUsersModal',
 *     searchEndpoint: '/admin/roles/{parentId}/users/search',
 *     membersEndpoint: '/admin/roles/{parentId}/users',
 *     addEndpoint: '/admin/roles/{parentId}/users',
 *     removeEndpoint: '/admin/roles/{parentId}/users/{entityId}',
 *     displayFields: { primary: 'login', secondary: 'first_name last_name' },
 *     config: { minChars: 2, debounceMs: 300, maxResults: 20, undoTimeoutMs: 5000 }
 *   });
 *
 *   // Open modal for a specific parent entity
 *   selector.open(parentId, parentName);
 */
class EntitySelector {
    constructor(options) {
        // Required options
        this.modalId = options.modalId;
        this.searchEndpoint = options.searchEndpoint;
        this.membersEndpoint = options.membersEndpoint;
        this.addEndpoint = options.addEndpoint || options.membersEndpoint;
        this.removeEndpoint = options.removeEndpoint;
        this.displayFields = options.displayFields || { primary: 'name', secondary: '' };

        // Configuration with defaults
        this.config = {
            minChars: options.config?.minChars || 2,
            debounceMs: options.config?.debounceMs || 300,
            maxResults: options.config?.maxResults || 20,
            undoTimeoutMs: options.config?.undoTimeoutMs || 5000
        };

        // i18n labels
        this.i18n = options.i18n || {
            singular: 'item',
            plural: 'items',
            added: 'Added',
            removed: 'Removed',
            undo: 'Undo',
            searchPlaceholder: 'Start typing to search...',
            filterPlaceholder: 'Filter members...',
            noMembers: 'No members yet',
            noResults: 'No results found',
            typeToSearch: 'Type at least {min} characters to search',
            pressEnter: 'Press Enter to add first result'
        };

        // State
        this.parentId = null;
        this.parentName = '';
        this.members = [];
        this.searchTimeout = null;
        this.undoTimeout = null;
        this.lastRemoved = null;
        this.isOpen = false;

        // Callbacks (optional)
        this.onMemberAdded = options.onMemberAdded || null;
        this.onMemberRemoved = options.onMemberRemoved || null;
        this.onError = options.onError || null;

        // DOM element cache
        this._elements = {};

        // Bind methods to preserve 'this' context
        this._handleKeydown = this._handleKeydown.bind(this);

        // Initialize if DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this._init());
        } else {
            this._init();
        }
    }

    // =========================================================================
    // Public API
    // =========================================================================

    /**
     * Open the modal for a specific parent entity
     * @param {number|string} parentId - The parent entity ID
     * @param {string} parentName - The parent entity name (for title display)
     */
    open(parentId, parentName = '') {
        this.parentId = parentId;
        this.parentName = parentName;
        this.isOpen = true;

        // Update title if element exists
        const titleEl = this._getElement('title');
        if (titleEl && parentName) {
            titleEl.textContent = parentName;
        }

        // Reset state
        this.members = [];
        this.lastRemoved = null;
        this._hideUndoToast();

        // Load members
        this._loadMembers();

        // Show modal
        const modal = this._getElement('modal');
        if (modal) {
            modal.classList.remove('hidden');
        }

        // Setup handlers
        this._setupSearch();
        this._setupMemberFilter();
        document.addEventListener('keydown', this._handleKeydown);

        // Focus search input
        setTimeout(() => {
            const searchInput = this._getElement('searchInput');
            if (searchInput) searchInput.focus();
        }, 100);
    }

    /**
     * Close the modal
     */
    close() {
        this.isOpen = false;

        // Remove keyboard handler
        document.removeEventListener('keydown', this._handleKeydown);

        // If there's a pending removal, execute it now
        if (this.undoTimeout && this.lastRemoved) {
            clearTimeout(this.undoTimeout);
            this._removeFromServer(this.lastRemoved.id);
            this.lastRemoved = null;
        }
        this._hideUndoToast();

        // Hide modal
        const modal = this._getElement('modal');
        if (modal) {
            modal.classList.add('hidden');
        }

        // Clear state
        this.parentId = null;
        this.parentName = '';
        this.members = [];
    }

    // =========================================================================
    // Private: Initialization
    // =========================================================================

    _init() {
        // Cache DOM elements
        this._cacheElements();
    }

    _cacheElements() {
        const prefix = this.modalId;
        this._elements = {
            modal: document.getElementById(prefix),
            title: document.getElementById(`${prefix}-title`),
            membersList: document.getElementById(`${prefix}-members-list`),
            memberCount: document.getElementById(`${prefix}-member-count`),
            memberFilter: document.getElementById(`${prefix}-member-filter`),
            searchInput: document.getElementById(`${prefix}-search`),
            searchSpinner: document.getElementById(`${prefix}-search-spinner`),
            searchResults: document.getElementById(`${prefix}-search-results`),
            undoToast: document.getElementById(`${prefix}-undo-toast`),
            undoMessage: document.getElementById(`${prefix}-undo-message`),
            undoBtn: document.getElementById(`${prefix}-undo-btn`)
        };
    }

    _getElement(name) {
        if (!this._elements[name]) {
            this._cacheElements();
        }
        return this._elements[name];
    }

    // =========================================================================
    // Private: Search
    // =========================================================================

    _setupSearch() {
        const searchInput = this._getElement('searchInput');
        if (!searchInput) return;

        // Clear value
        searchInput.value = '';
        this._resetSearchResults();

        // Remove existing listener by cloning
        const newInput = searchInput.cloneNode(true);
        searchInput.parentNode.replaceChild(newInput, searchInput);
        this._elements.searchInput = newInput;

        // Add debounced search
        newInput.addEventListener('input', () => {
            const query = newInput.value.trim();

            if (this.searchTimeout) {
                clearTimeout(this.searchTimeout);
            }

            if (query.length < this.config.minChars) {
                this._resetSearchResults();
                return;
            }

            this.searchTimeout = setTimeout(() => {
                this._searchEntities(query);
            }, this.config.debounceMs);
        });
    }

    _resetSearchResults() {
        const resultsList = this._getElement('searchResults');
        if (!resultsList) return;

        const minChars = this.config.minChars;
        resultsList.innerHTML = `
            <div class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                <svg class="mx-auto h-8 w-8 text-gray-300 dark:text-gray-600 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
                ${this.i18n.typeToSearch.replace('{min}', minChars)}
            </div>`;
    }

    _searchEntities(query) {
        const spinner = this._getElement('searchSpinner');
        const resultsList = this._getElement('searchResults');

        if (spinner) spinner.classList.remove('hidden');

        const url = this._buildUrl(this.searchEndpoint, { q: query });

        fetch(url, {
            headers: {
                'Accept': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            }
        })
        .then(response => this._handleResponse(response))
        .then(data => {
            if (spinner) spinner.classList.add('hidden');
            this._renderSearchResults(data);
        })
        .catch(error => {
            if (spinner) spinner.classList.add('hidden');
            console.error('Search error:', error);
            if (resultsList) {
                resultsList.innerHTML = `
                    <div class="px-4 py-4 text-center text-sm text-red-500 dark:text-red-400">
                        Search failed - please try again
                    </div>`;
            }
        });
    }

    _renderSearchResults(data) {
        const resultsList = this._getElement('searchResults');
        if (!resultsList) return;

        resultsList.innerHTML = '';

        // Handle different response formats
        const items = data.users || data.members || data.items || data.results || [];

        if (items.length > 0) {
            items.forEach(item => {
                const div = document.createElement('div');
                div.className = 'px-3 py-2 hover:bg-green-50 dark:hover:bg-green-900/20 cursor-pointer group transition-colors duration-150';
                div.dataset.entityId = this._getEntityId(item);
                div.innerHTML = `
                    <div class="flex items-center justify-between">
                        <div class="flex-1 min-w-0">
                            <p class="text-sm font-medium text-gray-900 dark:text-white truncate">${this._escapeHtml(this._getPrimaryDisplay(item))}</p>
                            <p class="text-xs text-gray-500 dark:text-gray-400 truncate">${this._escapeHtml(this._getSecondaryDisplay(item))}</p>
                        </div>
                        <div class="ml-2 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                            <span class="text-xs text-green-600 mr-1">Add</span>
                            <svg class="h-5 w-5 text-green-500 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                            </svg>
                        </div>
                    </div>
                `;
                div.onclick = () => this._addMember(item);
                resultsList.appendChild(div);
            });
        } else {
            resultsList.innerHTML = `
                <div class="px-4 py-4 text-center text-sm text-gray-500 dark:text-gray-400">
                    ${this.i18n.noResults}
                </div>`;
        }
    }

    // =========================================================================
    // Private: Members
    // =========================================================================

    _loadMembers() {
        const url = this._buildUrl(this.membersEndpoint);

        fetch(url, {
            headers: {
                'Accept': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            }
        })
        .then(response => this._handleResponse(response))
        .then(data => {
            // Handle different response formats
            this.members = data.members || data.users || data.items || [];
            this._renderMembers();
        })
        .catch(error => {
            console.error('Error loading members:', error);
            this._showError('Failed to load members');
        });
    }

    _renderMembers(highlightId = null) {
        const membersList = this._getElement('membersList');
        const memberCount = this._getElement('memberCount');

        if (memberCount) {
            memberCount.textContent = `(${this.members.length})`;
        }

        if (!membersList) return;
        membersList.innerHTML = '';

        if (this.members.length > 0) {
            this.members.forEach(item => {
                const entityId = this._getEntityId(item);
                const isHighlighted = entityId === highlightId;

                const div = document.createElement('div');
                div.className = `px-3 py-2 hover:bg-red-50 dark:hover:bg-red-900/20 cursor-pointer group transition-colors duration-150 ${isHighlighted ? 'bg-green-50 dark:bg-green-900/20' : ''}`;
                div.dataset.entityId = entityId;
                div.dataset.searchText = this._getSearchableText(item).toLowerCase();
                div.innerHTML = `
                    <div class="flex items-center justify-between">
                        <div class="flex-1 min-w-0">
                            <p class="text-sm font-medium text-gray-900 dark:text-white truncate">${this._escapeHtml(this._getPrimaryDisplay(item))}</p>
                            <p class="text-xs text-gray-500 dark:text-gray-400 truncate">${this._escapeHtml(this._getSecondaryDisplay(item))}</p>
                        </div>
                        <div class="ml-2 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                            <span class="text-xs text-red-500 mr-1">Remove</span>
                            <svg class="h-5 w-5 text-red-500 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </div>
                    </div>
                `;
                div.onclick = () => this._removeMember(item);
                membersList.appendChild(div);

                // Flash animation for newly added
                if (isHighlighted) {
                    setTimeout(() => div.classList.remove('bg-green-50', 'dark:bg-green-900/20'), 2000);
                }
            });
        } else {
            membersList.innerHTML = `
                <div class="px-4 py-6 text-center text-sm text-gray-500 dark:text-gray-400">
                    ${this.i18n.noMembers}
                </div>`;
        }
    }

    _setupMemberFilter() {
        const filterInput = this._getElement('memberFilter');
        if (!filterInput) return;

        filterInput.value = '';

        filterInput.addEventListener('input', () => {
            const query = filterInput.value.trim().toLowerCase();
            const membersList = this._getElement('membersList');
            if (!membersList) return;

            const items = membersList.querySelectorAll('[data-entity-id]');
            items.forEach(item => {
                const searchText = item.dataset.searchText || '';
                item.style.display = searchText.includes(query) ? '' : 'none';
            });
        });
    }

    // =========================================================================
    // Private: Add/Remove Operations
    // =========================================================================

    _addMember(entity) {
        const entityId = this._getEntityId(entity);
        const searchInput = this._getElement('searchInput');
        const currentQuery = searchInput ? searchInput.value : '';

        // Optimistically add to UI
        this.members.push(entity);
        this._renderMembers(entityId);

        // Remove from search results with animation
        const resultItem = this._getElement('searchResults')?.querySelector(`[data-entity-id="${entityId}"]`);
        if (resultItem) {
            resultItem.style.transition = 'opacity 0.2s, transform 0.2s';
            resultItem.style.opacity = '0';
            resultItem.style.transform = 'translateX(10px)';
            setTimeout(() => resultItem.remove(), 200);
        }

        // Add to server
        const url = this._buildUrl(this.addEndpoint);
        fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            },
            body: JSON.stringify(this._buildAddPayload(entityId))
        })
        .then(response => this._handleResponse(response))
        .then(data => {
            if (data.success !== false) {
                this._showToast('success', `${this.i18n.added} ${this._getPrimaryDisplay(entity)}`);

                // Re-run search to update results (keeps query)
                if (currentQuery.length >= this.config.minChars) {
                    this._searchEntities(currentQuery);
                }

                if (this.onMemberAdded) this.onMemberAdded(entity);
            } else {
                this._rollbackAdd(entityId);
                this._showToast('error', data.error || 'Failed to add');
            }
        })
        .catch(error => {
            console.error('Error adding member:', error);
            this._rollbackAdd(entityId);
            this._showToast('error', 'Failed to add');
        });
    }

    _rollbackAdd(entityId) {
        this.members = this.members.filter(m => this._getEntityId(m) !== entityId);
        this._renderMembers();
    }

    _removeMember(entity) {
        const entityId = this._getEntityId(entity);
        this.lastRemoved = { id: entityId, entity: entity };

        // Optimistically remove from UI with animation
        const membersList = this._getElement('membersList');
        const item = membersList?.querySelector(`[data-entity-id="${entityId}"]`);
        if (item) {
            item.style.transition = 'opacity 0.2s, transform 0.2s';
            item.style.opacity = '0';
            item.style.transform = 'translateX(-10px)';
            setTimeout(() => item.remove(), 200);
        }

        // Update cache
        this.members = this.members.filter(m => this._getEntityId(m) !== entityId);
        const memberCount = this._getElement('memberCount');
        if (memberCount) memberCount.textContent = `(${this.members.length})`;

        // Show undo toast
        this._showUndoToast(`${this.i18n.removed} ${this._getPrimaryDisplay(entity)}`, () => {
            // Undo: re-add
            this._addMemberQuiet(entity);
        });

        // Actually remove after delay (if not undone)
        if (this.undoTimeout) clearTimeout(this.undoTimeout);
        this.undoTimeout = setTimeout(() => {
            this._removeFromServer(entityId);
            this.lastRemoved = null;
            if (this.onMemberRemoved) this.onMemberRemoved(entity);
        }, this.config.undoTimeoutMs);
    }

    _addMemberQuiet(entity) {
        const entityId = this._getEntityId(entity);
        this.members.push(entity);
        this._renderMembers(entityId);

        const url = this._buildUrl(this.addEndpoint);
        fetch(url, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            },
            body: JSON.stringify(this._buildAddPayload(entityId))
        }).catch(error => console.error('Error restoring member:', error));
    }

    _removeFromServer(entityId) {
        const url = this._buildUrl(this.removeEndpoint, {}, entityId);
        fetch(url, {
            method: 'DELETE',
            headers: { 'X-Requested-With': 'XMLHttpRequest' }
        }).catch(error => console.error('Error removing member:', error));
    }

    // =========================================================================
    // Private: Undo Toast
    // =========================================================================

    _showUndoToast(message, undoCallback) {
        const toast = this._getElement('undoToast');
        const messageEl = this._getElement('undoMessage');
        const undoBtn = this._getElement('undoBtn');

        if (!toast) return;

        if (messageEl) messageEl.textContent = message;
        toast.classList.remove('hidden');

        if (undoBtn) {
            undoBtn.onclick = () => {
                if (this.undoTimeout) clearTimeout(this.undoTimeout);
                this._hideUndoToast();
                undoCallback();
            };
        }
    }

    _hideUndoToast() {
        const toast = this._getElement('undoToast');
        if (toast) toast.classList.add('hidden');
    }

    // =========================================================================
    // Private: Keyboard Navigation
    // =========================================================================

    _handleKeydown(e) {
        if (!this.isOpen) return;

        if (e.key === 'Escape') {
            this.close();
        }

        // Enter key adds first search result
        const searchInput = this._getElement('searchInput');
        if (e.key === 'Enter' && document.activeElement === searchInput) {
            const firstResult = this._getElement('searchResults')?.querySelector('[data-entity-id]');
            if (firstResult) {
                e.preventDefault();
                firstResult.click();
            }
        }
    }

    // =========================================================================
    // Private: Utilities
    // =========================================================================

    _buildUrl(template, queryParams = {}, entityId = null) {
        let url = template
            .replace('{parentId}', this.parentId)
            .replace(':parent_id', this.parentId)
            .replace('{entityId}', entityId)
            .replace(':entity_id', entityId);

        const params = new URLSearchParams(queryParams);
        if (params.toString()) {
            url += '?' + params.toString();
        }

        return url;
    }

    _buildAddPayload(entityId) {
        // Default payload - can be overridden
        return { user_id: entityId };
    }

    async _handleResponse(response) {
        const contentType = response.headers.get('content-type');
        if (!contentType || !contentType.includes('application/json')) {
            throw new Error('Server returned non-JSON response');
        }

        const data = await response.json();
        if (!response.ok) {
            throw new Error(data.error || `Server returned ${response.status}`);
        }

        return data;
    }

    _getEntityId(item) {
        return item.user_id || item.id || item.entity_id;
    }

    _getPrimaryDisplay(item) {
        const field = this.displayFields.primary;
        return item[field] || item.name || item.login || '';
    }

    _getSecondaryDisplay(item) {
        const fields = (this.displayFields.secondary || '').split(' ');
        return fields.map(f => item[f] || '').filter(Boolean).join(' ');
    }

    _getSearchableText(item) {
        return [
            this._getPrimaryDisplay(item),
            this._getSecondaryDisplay(item)
        ].join(' ');
    }

    _escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    _showToast(type, message) {
        // Use global showToast if available (from common.js)
        if (typeof showToast === 'function') {
            showToast(type, message);
        } else if (typeof window.showToast === 'function') {
            window.showToast(type, message);
        } else {
            // Fallback: log to console
            console.log(`[${type}] ${message}`);
        }
    }

    _showError(message) {
        if (this.onError) {
            this.onError(message);
        } else {
            this._showToast('error', message);
        }
    }
}

// Export for global use
window.EntitySelector = EntitySelector;
