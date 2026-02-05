/**
 * Ticket Zoom JavaScript Functions - FIXED VERSION
 * Fixes: Add Note button functionality with existing modal system
 */

// Global state
let currentTicketId = null;
let isComposing = false;

function getPendingStateSet(container) {
    const raw = (container && container.dataset && container.dataset.pendingStates) || '';
    const tokens = raw.split(',').map(token => token.trim()).filter(Boolean);
    if (tokens.length === 0) {
        return new Set(['4', '5', '6', '7', '8']);
    }
    return new Set(tokens);
}

/**
 * Initialize ticket zoom functionality
 */
function initTicketZoom(ticketId) {
    currentTicketId = ticketId;
    console.log('Ticket zoom initialized for ticket ID:', ticketId);
    
    // Initialize keyboard shortcuts
    initializeKeyboardShortcuts();
    
    // Initialize auto-save for drafts
    initializeAutoSave();
}

/**
 * Add internal note to ticket - FIXED to work with template modals
 * Fixes: UNCAUGHT REFERENCEERROR: ADDNOTE IS NOT DEFINED
 */
function addNote(ticketId) {
    if (!ticketId) ticketId = currentTicketId;
    if (!ticketId) {
        console.error('No ticket ID provided for addNote');
        return;
    }
    
    console.log('Adding note to ticket:', ticketId);
    
    // FIXED: Use existing template modal instead of custom composer
    const noteModal = document.getElementById('noteModal');
    if (noteModal) {
        // Show the modal by removing 'hidden' class
        noteModal.classList.remove('hidden');
        
        // Reset the note type dropdown to Internal
        const noteTypeSelect = document.getElementById('noteTypeSelect');
        if (noteTypeSelect) {
            noteTypeSelect.value = '3'; // Internal
        }
        
        // Clear any existing content
        const subjectField = noteModal.querySelector('[name="subject"]');
        const bodyField = noteModal.querySelector('[name="body"]');
        
        if (subjectField) {
            subjectField.value = 'Internal Note';
        }
        if (bodyField) {
            bodyField.value = '';
            bodyField.focus();
        }
        
        isComposing = true;
        console.log('Note modal opened');
    } else {
        console.error('Note modal not found');
    }
}

/**
 * FIXED: Update other modal-related functions to match template system
 */
function replyToTicket() {
    const replyModal = document.getElementById('replyModal');
    if (replyModal) {
        replyModal.classList.remove('hidden');
        const bodyField = replyModal.querySelector('[name="body"]');
        if (bodyField) {
            bodyField.focus();
        }
        isComposing = true;
        console.log('Reply modal opened');
        
        // Load customer users for dropdown
        loadCustomerUsers();
        
        // Load queue signature and append to editor
        loadQueueSignature();
    }
}

/**
 * Load the queue's signature and append to the reply editor
 */
function loadQueueSignature() {
    const queueId = getCurrentQueueId();
    const ticketId = currentTicketId;
    
    if (!queueId) return;
    
    const url = `/agent/api/signatures/queue/${queueId}` + 
                (ticketId ? `?ticket_id=${ticketId}` : '');
    
    apiFetch(url)
        
        .then(data => {
            if (data.success && data.data && data.data.text) {
                const signatureText = data.data.text;
                const contentType = data.data.content_type || 'text/html';
                const targetEditorId = 'replyEditorInner';
                
                // Use global TiptapEditor API to append signature
                if (window.TiptapEditor && typeof window.TiptapEditor.getContent === 'function') {
                    // Switch editor mode based on content type BEFORE setting content
                    const targetMode = contentType === 'text/plain' ? 'markdown' : 'richtext';
                    if (typeof window.TiptapEditor.setMode === 'function') {
                        window.TiptapEditor.setMode(targetEditorId, targetMode);
                    }
                    
                    const currentContent = window.TiptapEditor.getContent(targetEditorId) || '';
                    
                    // Append signature with separator
                    const separator = contentType === 'text/plain' ? '\n\n-- \n' : '<br><br><hr>';
                    
                    if (currentContent.trim() && currentContent.trim() !== '<p></p>') {
                        window.TiptapEditor.setContent(targetEditorId, currentContent + separator + signatureText);
                    } else {
                        // Start with empty content + signature
                        window.TiptapEditor.setContent(targetEditorId, separator + signatureText);
                    }
                    console.log('Signature appended to editor');
                } else {
                    // Fallback to textarea
                    const textarea = document.querySelector('#replyModal textarea[name="body"]');
                    if (textarea) {
                        const currentContent = textarea.value || '';
                        const separator = '\n\n-- \n';
                        if (currentContent.trim()) {
                            textarea.value = currentContent + separator + signatureText;
                        } else {
                            textarea.value = separator + signatureText;
                        }
                        console.log('Signature appended to textarea');
                    }
                }
            }
        })
        .catch(err => {
            console.error('Failed to load queue signature:', err);
        });
}

/**
 * Load customer users for the reply dropdown
 */
function loadCustomerUsers() {
    const ticketId = currentTicketId;
    const selectElement = document.getElementById('customerUserSelect');
    const helpText = selectElement ? selectElement.nextElementSibling : null;
    
    if (!selectElement || !ticketId) return;
    
    // Update help text
    if (helpText) {
        helpText.textContent = 'Loading customer users...';
    }
    
    apiFetch(`/agent/tickets/${ticketId}/customer-users`)
        
        .then(data => {
            if (data.success && data.customer_users) {
                // Clear existing options
                selectElement.innerHTML = '';
                
                // Add customer users as options
                data.customer_users.forEach(user => {
                    const option = document.createElement('option');
                    option.value = user.email || user.login;
                    option.textContent = user.display_name || user.email || user.login;
                    
                    // Select current customer by default
                    if (user.is_current) {
                        option.selected = true;
                    }
                    
                    selectElement.appendChild(option);
                });
                
                // Add option to enter custom email
                const customOption = document.createElement('option');
                customOption.value = '__custom__';
                customOption.textContent = '-- Enter custom email address --';
                selectElement.appendChild(customOption);
                
                // Update help text
                if (helpText) {
                    if (data.customer_users.length > 0) {
                        helpText.textContent = `${data.customer_users.length} customer user(s) available`;
                    } else {
                        helpText.textContent = 'No customer users found for this company';
                    }
                }
            }
        })
        .catch(error => {
            console.error('Error loading customer users:', error);
            if (helpText) {
                helpText.textContent = 'Error loading customer users';
            }
        });
}

/**
 * FIXED: Close any modal function
 */
function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.classList.add('hidden');
        isComposing = false;
        console.log(`Modal ${modalId} closed`);
    }
}

/**
 * Change ticket status
 */
function changeStatus() {
    // Build URL with optional ticket attribute relation filtering
    let statesUrl = '/api/v1/states';
    const queueName = getCurrentQueueName();
    if (queueName) {
        statesUrl += `?filter_attribute=Queue&filter_value=${encodeURIComponent(queueName)}`;
    }

    // Load available statuses (filtered by ticket attribute relations if applicable)
    apiFetch(statesUrl)
        
        .then(data => {
            const select = document.querySelector('#statusModal select[name="status_id"]');
            select.innerHTML = '<option value="">Select status...</option>';
            if (data.success && data.data) {
                data.data.forEach(status => {
                    const currentStatusId = getCurrentStatusId();
                    const selected = status.id == currentStatusId ? 'selected' : '';
                    select.innerHTML += `<option value="${status.id}" ${selected}>${status.name}</option>`;
                });
                console.log(`Loaded ${data.data.length} statuses`);
            } else {
                console.error('Failed to load statuses:', data);
            }
        })
        .catch(error => {
            console.error('Error loading statuses:', error);
            const select = document.querySelector('#statusModal select[name="status_id"]');
            select.innerHTML = '<option value="">Failed to load statuses - API error</option>';
            showToast('Failed to load statuses - check API connection', 'error');
        });
        
    const statusModal = document.getElementById('statusModal');
    if (statusModal) {
        statusModal.classList.remove('hidden');
        console.log('Status modal opened');
        
        // Add event listener to show/hide pending time field
        const statusForm = statusModal.querySelector('form');
        const pendingStates = getPendingStateSet(statusForm);
        const statusSelect = document.querySelector('#statusModal select[name="status_id"]');
        if (statusSelect) {
            statusSelect.addEventListener('change', function() {
                const pendingContainer = document.getElementById('pendingTimeContainer');
                const pendingInput = document.querySelector('#statusModal input[name="pending_until"]');
                const needsPending = pendingStates.has(this.value);
                if (pendingContainer && pendingInput) {
                    pendingContainer.style.display = needsPending ? 'block' : 'none';
                    pendingInput.required = needsPending;
                    if (needsPending && !pendingInput.value) {
                        const now = new Date(Date.now() + 60 * 60 * 1000);
                        const local = new Date(now.getTime() - now.getTimezoneOffset() * 60000);
                        pendingInput.value = local.toISOString().slice(0, 16);
                    }
                    if (!needsPending) {
                        pendingInput.value = '';
                    }
                }
            });
        }
    }
}

/**
 * Assign ticket to agent
 */
function assignAgent() {
    // Load available agents first
    const ticketQueueId = document.querySelector('[data-queue-id]')?.getAttribute('data-queue-id') || 1;
    console.log('Loading agents for queue:', ticketQueueId);
    
    apiFetch(`/api/v1/queues/${ticketQueueId}/agents`)
        
        .then(data => {
            console.log('Agents API response:', data);
            const select = document.querySelector('#assignModal select[name="user_id"]');
            select.innerHTML = '<option value="">Select agent...</option>';
            if (data.success && data.agents) {
                data.agents.forEach(agent => {
                    select.innerHTML += `<option value="${agent.id}">${agent.name}</option>`;
                });
                console.log('Populated select with', data.agents.length, 'agents');
            } else {
                console.warn('No agents returned from API');
            }
        })
        .catch(error => {
            console.error('API call failed:', error);
            // No fallback - fail hard so we know the API is broken
            const select = document.querySelector('#assignModal select[name="user_id"]');
            select.innerHTML = '<option value="">Failed to load agents - API error</option>';
            showToast('Failed to load agents - check API connection', 'error');
        });
        
    const assignModal = document.getElementById('assignModal');
    if (assignModal) {
        assignModal.classList.remove('hidden');
    } else {
        console.error('Assign modal not found');
    }
}

/**
 * Change ticket priority
 */
function changePriority() {
    // Build URL with optional ticket attribute relation filtering
    // Priority can be filtered by State (e.g., "new" state only allows certain priorities)
    let prioritiesUrl = '/api/v1/priorities';
    const stateName = getCurrentStateName();
    if (stateName) {
        prioritiesUrl += `?filter_attribute=State&filter_value=${encodeURIComponent(stateName)}`;
    }

    // Load available priorities (filtered by ticket attribute relations if applicable)
    apiFetch(prioritiesUrl)
        
        .then(data => {
            const select = document.querySelector('#priorityModal select[name="priority_id"]');
            select.innerHTML = '<option value="">Select priority...</option>';
            if (data.success && data.data) {
                data.data.forEach(priority => {
                    const currentPriorityId = getCurrentPriorityId();
                    const selected = priority.id == currentPriorityId ? 'selected' : '';
                    select.innerHTML += `<option value="${priority.id}" ${selected}>${priority.name}</option>`;
                });
                console.log(`Loaded ${data.data.length} priorities`);
            } else {
                console.error('Failed to load priorities:', data);
            }
        })
        .catch(error => {
            console.error('Error loading priorities:', error);
            // No fallback - fail hard so we know the API is broken
            const select = document.querySelector('#priorityModal select[name="priority_id"]');
            select.innerHTML = '<option value="">Failed to load priorities - API error</option>';
            showToast('Failed to load priorities - check API connection', 'error');
        });
        
    const priorityModal = document.getElementById('priorityModal');
    if (priorityModal) {
        priorityModal.classList.remove('hidden');
        console.log('Priority modal opened');
    }
}

/**
 * FIXED: Move ticket to queue with proper API integration
 */
function moveQueue() {
    // Load available queues
    apiFetch('/api/v1/queues')
        
        .then(data => {
            const select = document.querySelector('#queueModal select[name="queue_id"]');
            select.innerHTML = '<option value="">Select queue...</option>';
            if (data.success && data.data) {
                data.data.forEach(queue => {
                    const currentQueueId = getCurrentQueueId();
                    const selected = queue.id == currentQueueId ? 'selected' : '';
                    select.innerHTML += `<option value="${queue.id}" ${selected}>${queue.name}</option>`;
                });
                console.log(`Loaded ${data.data.length} queues`);
            } else {
                console.error('Failed to load queues:', data);
            }
        })
        .catch(error => {
            console.error('Error loading queues:', error);
            // No fallback - fail hard so we know the API is broken
            const select = document.querySelector('#queueModal select[name="queue_id"]');
            select.innerHTML = '<option value="">Failed to load queues - API error</option>';
            showToast('Failed to load queues - check API connection', 'error');
        });
        
    const queueModal = document.getElementById('queueModal');
    if (queueModal) {
        queueModal.classList.remove('hidden');
        console.log('Queue modal opened');
    }
}

/**
 * Get current queue ID from page data
 */
function getCurrentQueueId() {
    // Try to get from data attribute or global variable
    const queueElement = document.querySelector('[data-queue-id]');
    if (queueElement) {
        return queueElement.getAttribute('data-queue-id');
    }
    
    // Fallback to parsing from template if available
    if (window.ticketData && window.ticketData.queue_id) {
        return window.ticketData.queue_id;
    }
    
    return 1; // Default fallback
}

/**
 * Get current priority ID from page data
 */
function getCurrentPriorityId() {
    // Try to get from data attribute or global variable
    const priorityElement = document.querySelector('[data-priority-id]');
    if (priorityElement) {
        return priorityElement.getAttribute('data-priority-id');
    }
    
    // Fallback to parsing from template if available
    if (window.ticketData && window.ticketData.priority_id) {
        return window.ticketData.priority_id;
    }
    
    return 3; // Default fallback (usually "normal" priority)
}

/**
 * Get current ticket status ID
 */
function getCurrentStatusId() {
    // Try to get from data attribute
    const statusElement = document.querySelector('[data-status-id]');
    if (statusElement) {
        return statusElement.getAttribute('data-status-id');
    }

    // Fallback to parsing from template if available
    if (window.ticketData && window.ticketData.status_id) {
        return window.ticketData.status_id;
    }

    return 1; // Default fallback (usually "new" status)
}

/**
 * Get current queue name from page data (for ticket attribute relations filtering)
 */
function getCurrentQueueName() {
    // Try to get from data attribute
    const queueElement = document.querySelector('[data-queue-name]');
    if (queueElement) {
        return queueElement.getAttribute('data-queue-name');
    }

    // Fallback to parsing from template if available
    if (window.ticketData && window.ticketData.queue_name) {
        return window.ticketData.queue_name;
    }

    return null; // No fallback - relations won't filter if queue name unknown
}

/**
 * Get current state name from page data (for ticket attribute relations filtering)
 */
function getCurrentStateName() {
    // Try to get from data attribute
    const statusElement = document.querySelector('[data-status-name]');
    if (statusElement) {
        return statusElement.getAttribute('data-status-name');
    }

    // Fallback to parsing from template if available
    if (window.ticketData && window.ticketData.status_name) {
        return window.ticketData.status_name;
    }

    return null; // No fallback - relations won't filter if state name unknown
}

/**
 * Merge ticket
 */
function mergeTicket() {
    const mergeModal = document.getElementById('mergeModal');
    if (mergeModal) {
        mergeModal.classList.remove('hidden');
        console.log('Merge modal opened');
    }
}

/**
 * FIXED: Form submission handlers using existing template forms
 */
function submitReply(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    apiFetch(`/agent/tickets/${currentTicketId}/reply`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Reply sent successfully', 'success');
            closeModal('replyModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to send reply: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error sending reply:', error);
        showToast('Error sending reply', 'error');
    });
}

function submitNote(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    apiFetch(`/agent/tickets/${currentTicketId}/note`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Note added successfully', 'success');
            closeModal('noteModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to add note: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error adding note:', error);
        showToast('Error adding note', 'error');
    });
}

function submitStatus(event) {
    event.preventDefault();
    const form = event.target;
    const statusSelect = form.querySelector('select[name="status_id"], select[name="status"]');
    const pendingInput = form.querySelector('input[name="pending_until"]');
    const pendingStates = getPendingStateSet(form);
    const statusValue = statusSelect ? String(statusSelect.value).trim() : '';
    const requiresPending = statusValue && pendingStates.has(statusValue);

    if (requiresPending) {
        const value = pendingInput ? String(pendingInput.value).trim() : '';
        if (!value) {
            showToast('Pending states require a follow-up time.', 'error');
            if (pendingInput && typeof pendingInput.focus === 'function') {
                pendingInput.focus();
            }
            return;
        }
        if (Number.isNaN(Date.parse(value))) {
            showToast('Enter a valid pending time.', 'error');
            if (pendingInput && typeof pendingInput.focus === 'function') {
                pendingInput.focus();
            }
            return;
        }
    }

    const formData = new FormData(form);
    if (!requiresPending) {
        formData.delete('pending_until');
    }
    
    apiFetch(`/agent/tickets/${currentTicketId}/status`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Status updated successfully', 'success');
            closeModal('statusModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to update status: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error updating status:', error);
        showToast('Error updating status', 'error');
    });
}

function submitAssign(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    // Log the form data for debugging
    console.log('Submitting assignment form data:');
    for (let [key, value] of formData.entries()) {
        console.log(`${key}: ${value}`);
    }
    
    apiFetch(`/agent/tickets/${currentTicketId}/assign`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Ticket assigned successfully', 'success');
            closeModal('assignModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to assign ticket: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error assigning ticket:', error);
        showToast('Error assigning ticket', 'error');
    });
}

function submitPriority(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    apiFetch(`/agent/tickets/${currentTicketId}/priority`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Priority updated successfully', 'success');
            closeModal('priorityModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to update priority: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error updating priority:', error);
        showToast('Error updating priority', 'error');
    });
}

function submitQueue(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    apiFetch(`/agent/tickets/${currentTicketId}/queue`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Ticket moved successfully', 'success');
            closeModal('queueModal');
            setTimeout(() => location.reload(), 1000);
        } else {
            showToast('Failed to move ticket: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error moving ticket:', error);
        showToast('Error moving ticket', 'error');
    });
}

function submitMerge(event) {
    event.preventDefault();
    const formData = new FormData(event.target);
    
    apiFetch(`/agent/tickets/${currentTicketId}/merge`, {
        method: 'POST',
        body: formData
    })
    
    .then(data => {
        if (data.success) {
            showToast('Tickets merged successfully', 'success');
            closeModal('mergeModal');
            setTimeout(() => location.href = '/tickets', 1000);
        } else {
            showToast('Failed to merge tickets: ' + (data.error || 'Unknown error'), 'error');
        }
    })
    .catch(error => {
        console.error('Error merging tickets:', error);
        showToast('Error merging tickets', 'error');
    });
}

/**
 * Load attachment images with authentication
 */
function loadAttachmentImages() {
    document.querySelectorAll('img[data-attachment-url]').forEach(img => {
        const url = img.getAttribute('data-attachment-url');
        fetch(url, {
            credentials: 'include',
            headers: {
                'Accept': 'image/*'
            }
        })
        .then(response => {
            if (response.ok) {
                return response.blob();
            } else {
                throw new Error('Failed to load image');
            }
        })
        .then(blob => {
            const objectUrl = URL.createObjectURL(blob);
            img.src = objectUrl;
            img.onload = () => URL.revokeObjectURL(objectUrl);
        })
        .catch(error => {
            console.error('Failed to load attachment image:', error);
            // Hide the broken image and show a placeholder
            img.style.display = 'none';
            const placeholder = document.createElement('div');
            placeholder.className = 'flex items-center justify-center h-full bg-gray-200 dark:bg-gray-700 text-gray-400';
            placeholder.innerHTML = '<svg class="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"></path></svg>';
            img.parentElement.appendChild(placeholder);
        });
    });
}

/**
 * Initialize keyboard shortcuts
 */
function initializeKeyboardShortcuts() {
    // Helper: determine if the event target is inside an editor or editable field
    function isEditableTarget(target) {
        if (!target) return false;
        // Standard inputs
        const tag = target.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || tag === 'BUTTON') return true;
        if (target.isContentEditable) return true;
        // Tiptap/ProseMirror/editor containers
        if (target.closest('[contenteditable="true"], .ProseMirror, .tiptap, #noteEditor, #bodyEditor')) return true;
        return false;
    }

    function shouldIgnoreShortcut(e) {
        // Ignore when composing (IME), or when focus is in any editable context
        if (e.isComposing) return true;
        if (isEditableTarget(e.target)) return true;
        // Also check activeElement in case event target is a child of the editor
        const ae = document.activeElement;
        if (ae && (ae.isContentEditable || isEditableTarget(ae))) return true;
        return false;
    }

    document.addEventListener('keydown', function(e) {
        // Only handle shortcuts when not typing in editors/fields
        if (shouldIgnoreShortcut(e)) return;

        const key = (e.key || '').toLowerCase();
        switch(key) {
            case 'r':
                if (!isComposing) {
                    // Only act when a modifier is held to avoid clobbering typing
                    const withModifier = e.ctrlKey || e.metaKey || e.altKey || e.shiftKey;
                    if (!withModifier) return;
                    const hasReplyModal = !!document.getElementById('replyModal');
                    if (hasReplyModal) {
                        e.preventDefault();
                        replyToTicket();
                    }
                    // If no reply modal on this page, don't prevent typing or other handlers
                }
                break;
            case 'n':
                if (!isComposing) {
                    // Only act when a modifier is held to avoid clobbering typing
                    const withModifier = e.ctrlKey || e.metaKey || e.altKey || e.shiftKey;
                    if (!withModifier) return;
                    const hasNoteModal = !!document.getElementById('noteModal');
                    if (hasNoteModal) {
                        e.preventDefault();
                        addNote(currentTicketId);
                        break;
                    }
                    // Fallback: focus inline note editor if present on this page
                    const noteEditorRoot = document.getElementById('noteEditor');
                    if (noteEditorRoot) {
                        const editable = noteEditorRoot.querySelector('[contenteditable="true"], .ProseMirror');
                        if (editable && typeof editable.focus === 'function') {
                            e.preventDefault();
                            editable.focus();
                            isComposing = true;
                        }
                    }
                    // Else, do not prevent default so typing continues
                }
                break;
            case 'escape':
                if (isComposing) {
                    e.preventDefault();
                    // Close any open modal
                    document.querySelectorAll('[id$="Modal"]').forEach(modal => {
                        modal.classList.add('hidden');
                    });
                    isComposing = false;
                }
                break;
        }
    });
}

/**
 * Initialize auto-save functionality
 */
function initializeAutoSave() {
    // Auto-save drafts every 30 seconds
    setInterval(() => {
        if (isComposing) {
            saveDraft();
        }
    }, 30000);
}

/**
 * Save draft - placeholder for future implementation
 */
function saveDraft() {
    console.log('Auto-saving draft...');
}

// Close modals on Escape key (duplicate from template - keeping for compatibility)  
document.addEventListener('keydown', function(event) {
    // Don't steal Escape while typing in editors/fields
    const target = event.target;
    const inEditor = target && (target.isContentEditable || target.closest('[contenteditable="true"], .ProseMirror, .tiptap, #noteEditor, #bodyEditor'));
    if (!inEditor && event.key === 'Escape') {
        document.querySelectorAll('[id$="Modal"]').forEach(modal => {
            modal.classList.add('hidden');
        });
        isComposing = false;
    }
});

// Handle custom email option in customer user dropdown
document.addEventListener('change', function(event) {
    if (event.target && event.target.id === 'customerUserSelect') {
        if (event.target.value === '__custom__') {
            // Replace select with input field
            const select = event.target;
            const parent = select.parentElement;
            
            const input = document.createElement('input');
            input.type = 'email';
            input.name = 'to';
            input.required = true;
            input.className = select.className;
            input.placeholder = 'Enter email address';
            input.id = 'customerUserInput';
            
            // Add back button to return to dropdown
            const backBtn = document.createElement('button');
            backBtn.type = 'button';
            backBtn.textContent = '← Back to list';
            backBtn.className = 'text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400 mt-1 block';
            backBtn.onclick = function() {
                input.replaceWith(select);
                backBtn.remove();
                loadCustomerUsers();
            };
            
            select.replaceWith(input);
            const helpText = parent.querySelector('.text-xs');
            if (helpText) {
                helpText.after(backBtn);
                helpText.textContent = 'Enter a custom email address';
            } else {
                input.after(backBtn);
            }
            input.focus();
        }
    }
});
