/**
 * Ticket Zoom JavaScript Functions - FIXED VERSION
 * Fixes: Add Note button functionality with existing modal system
 */

// Global state
let currentTicketId = null;
let isComposing = false;

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
    }
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
    const statusModal = document.getElementById('statusModal');
    if (statusModal) {
        statusModal.classList.remove('hidden');
        console.log('Status modal opened');
    }
}

/**
 * Assign ticket to agent
 */
function assignAgent() {
    // Load available agents first
    const ticketQueueId = document.querySelector('[data-queue-id]')?.getAttribute('data-queue-id') || 1;
    
    fetch(`/api/v1/queues/${ticketQueueId}/agents`)
        .then(r => r.json())
        .then(data => {
            const select = document.querySelector('#assignModal select[name="user_id"]');
            select.innerHTML = '<option value="">Select agent...</option>';
            if (data.agents) {
                data.agents.forEach(agent => {
                    select.innerHTML += `<option value="${agent.id}">${agent.name}</option>`;
                });
            }
        })
        .catch(() => {
            // No fallback - fail hard so we know the API is broken
            const select = document.querySelector('#assignModal select[name="user_id"]');
            select.innerHTML = '<option value="">Failed to load agents - API error</option>';
        });
        
    const assignModal = document.getElementById('assignModal');
    if (assignModal) {
        assignModal.classList.remove('hidden');
        console.log('Assign modal opened');
    }
}

/**
 * Change ticket priority
 */
function changePriority() {
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
    fetch('/api/v1/queues')
        .then(r => r.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/reply`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/note`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    const formData = new FormData(event.target);
    
    fetch(`/agent/tickets/${currentTicketId}/status`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/assign`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/priority`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/queue`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
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
    
    fetch(`/agent/tickets/${currentTicketId}/merge`, {
        method: 'POST',
        body: formData
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            showToast('Tickets merged successfully', 'success');
            closeModal('mergeModal');
            setTimeout(() => location.href = '/agent/tickets', 1000);
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
 * Show toast notification
 */
function showToast(message, type) {
    // Create toast element if it doesn't exist
    let toast = document.getElementById('toast-notification');
    if (!toast) {
        toast = document.createElement('div');
        toast.id = 'toast-notification';
        toast.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            padding: 15px 20px;
            border-radius: 4px;
            color: white;
            font-weight: bold;
            z-index: 9999;
            opacity: 0;
            transition: opacity 0.3s ease;
        `;
        document.body.appendChild(toast);
    }
    
    // Set message and type
    toast.textContent = message;
    toast.className = `toast-${type}`;
    
    // Set background color based on type
    const colors = {
        success: '#22c55e',
        error: '#ef4444',
        warning: '#f59e0b',
        info: '#3b82f6'
    };
    toast.style.backgroundColor = colors[type] || colors.info;
    
    // Show toast
    toast.style.opacity = '1';
    
    // Hide after 3 seconds
    setTimeout(() => {
        toast.style.opacity = '0';
    }, 3000);
}

/**
 * Initialize keyboard shortcuts
 */
function initializeKeyboardShortcuts() {
    document.addEventListener('keydown', function(e) {
        // Only handle shortcuts when not in input fields
        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
            return;
        }
        
        switch(e.key.toLowerCase()) {
            case 'r':
                if (!isComposing) {
                    e.preventDefault();
                    replyToTicket();
                }
                break;
            case 'n':
                if (!isComposing) {
                    e.preventDefault();
                    addNote(currentTicketId);
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
    if (event.key === 'Escape') {
        document.querySelectorAll('[id$="Modal"]').forEach(modal => {
            modal.classList.add('hidden');
        });
        isComposing = false;
    }
});