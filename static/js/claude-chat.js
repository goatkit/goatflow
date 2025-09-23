// Claude Code Context-Aware Chat Component
// Provides direct feedback channel with full page context

(function() {
    'use strict';

    // Configuration
    const CHAT_CONFIG = {
        position: 'bottom-right',
        expandedWidth: '400px',
        expandedHeight: '600px',
        collapsedSize: '60px',
        animationDuration: 300
    };

    // Chat state
    let chatState = {
        isOpen: false,
        messages: [],
        websocket: null,
        sessionId: generateSessionId(),
        isConnected: false,
        isTyping: false,
        activeTickets: [],
        ticketPollInterval: null,
        context: {
            page: window.location.pathname,
            url: window.location.href,
            timestamp: new Date().toISOString(),
            userAgent: navigator.userAgent,
            screenResolution: `${window.screen.width}x${window.screen.height}`,
            viewportSize: `${window.innerWidth}x${window.innerHeight}`,
            user: null, // Will be populated from page
            mousePosition: { x: 0, y: 0 },
            hoveredElement: null,
            selectedElement: null
        }
    };

    // Initialize on DOM ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', initializeChatbot);
    } else {
        initializeChatbot();
    }

    function initializeChatbot() {
        createChatUI();
        attachEventListeners();
        loadChatHistory();
        collectPageContext();
        // Connect WebSocket when chat opens
    }
    
    function generateSessionId() {
        return 'session-' + Date.now() + '-' + Math.random().toString(36).substr(2, 9);
    }

    function createChatUI() {
        // Create main container
        const chatContainer = document.createElement('div');
        chatContainer.id = 'claude-chat-container';
        chatContainer.className = 'fixed z-50 transition-all duration-300';
        chatContainer.innerHTML = `
            <!-- Collapsed State - Floating Button -->
            <div id="claude-chat-button" class="chat-collapsed">
                <button class="bg-gotrs-600 hover:bg-gotrs-700 text-white rounded-full p-4 shadow-lg hover:shadow-xl transition-all relative">
                    <!-- Chat Bubble Icon -->
                    <svg class="w-7 h-7" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
                    </svg>
                    <span id="chat-notification" class="hidden absolute -top-1 -right-1 bg-red-500 text-white text-xs rounded-full h-5 w-5 flex items-center justify-center animate-bounce">!</span>
                </button>
                <div class="absolute bottom-full right-0 mb-2 px-3 py-1 bg-gray-800 text-white text-sm rounded-lg opacity-0 hover:opacity-100 transition-opacity whitespace-nowrap">
                    Report issue to Claude Code
                </div>
            </div>

            <!-- Expanded State - Chat Window -->
            <div id="claude-chat-window" class="chat-expanded hidden">
                <div class="bg-white dark:bg-gray-800 rounded-lg shadow-2xl flex flex-col h-full">
                    <!-- Header -->
                    <div class="bg-gotrs-600 text-white px-4 py-3 rounded-t-lg flex items-center justify-between">
                        <div class="flex items-center space-x-2">
                            <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                      d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z">
                                </path>
                            </svg>
                            <span class="font-semibold">Claude Code Assistant</span>
                        </div>
                        <div class="flex items-center space-x-2">
                            <button id="chat-context-btn" class="hover:bg-gotrs-700 p-1 rounded transition-colors" title="Show context">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                                          d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z">
                                    </path>
                                </svg>
                            </button>
                            <button id="chat-minimize-btn" class="hover:bg-gotrs-700 p-1 rounded transition-colors">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path>
                                </svg>
                            </button>
                        </div>
                    </div>

                    <!-- Context Bar (Shown by default) -->
                    <div id="chat-context-bar" class="bg-blue-50 dark:bg-gray-700 px-4 py-2 text-xs border-b dark:border-gray-600">
                        <div class="flex items-center justify-between">
                            <span class="text-gray-600 dark:text-gray-300">
                                Page: <span class="font-mono text-blue-600 dark:text-blue-400">${window.location.pathname}</span>
                            </span>
                            <button id="chat-select-element" class="bg-blue-500 hover:bg-blue-600 text-white px-2 py-1 rounded text-xs">
                                🎯 Select Element
                            </button>
                        </div>
                        <div id="selected-element-info" class="mt-1 text-gray-600 dark:text-gray-300 hidden">
                            Selected: <span class="font-mono text-green-600 dark:text-green-400"></span>
                        </div>
                    </div>

                    <!-- Messages Area -->
                    <div id="chat-messages" class="flex-1 overflow-y-auto p-4 space-y-3">
                        <div class="text-center text-gray-500 dark:text-gray-400 text-sm">
                            <p>👋 Hi! I'm Claude Code. I can see you're on:</p>
                            <p class="font-mono mt-1 text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded inline-block">
                                ${window.location.pathname}
                            </p>
                            <p class="mt-2">Report any issues or suggest improvements!</p>
                            <p class="text-xs mt-1">💡 Tip: Click "Select Element" to point at specific UI problems</p>
                        </div>
                    </div>

                    <!-- Input Area with Multi-line Support -->
                    <div class="border-t dark:border-gray-700 p-4">
                        <div class="flex space-x-2 items-end">
                            <textarea 
                                id="chat-input" 
                                rows="1"
                                class="flex-1 px-3 py-2 border dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-gotrs-500 dark:bg-gray-700 dark:text-white resize-none overflow-hidden"
                                placeholder="Describe the issue or suggestion..."
                                autocomplete="off"
                                style="min-height: 40px; max-height: 120px;"
                            ></textarea>
                            <button 
                                id="chat-send-btn"
                                class="bg-gotrs-600 hover:bg-gotrs-700 text-white px-4 py-2 rounded-lg transition-colors mb-0"
                            >
                                Send
                            </button>
                        </div>
                        <div class="mt-2 flex items-center justify-between text-xs text-gray-500 dark:text-gray-400">
                            <span>Press Enter to send, Shift+Enter for new line</span>
                            <span id="chat-status">Ready</span>
                        </div>
                    </div>
                </div>
            </div>
        `;

        // Add styles
        const styles = document.createElement('style');
        styles.innerHTML = `
            #claude-chat-container {
                bottom: 20px;
                right: 20px;
            }

            .chat-collapsed {
                display: block;
            }

            .chat-expanded {
                width: ${CHAT_CONFIG.expandedWidth};
                height: ${CHAT_CONFIG.expandedHeight};
                max-width: calc(100vw - 40px);
                max-height: calc(100vh - 40px);
            }

            #claude-chat-window {
                animation: slideUp 0.3s ease-out;
            }

            @keyframes slideUp {
                from {
                    opacity: 0;
                    transform: translateY(20px);
                }
                to {
                    opacity: 1;
                    transform: translateY(0);
                }
            }

            #chat-input {
                line-height: 1.5;
                transition: height 0.1s ease;
            }
            
            #chat-input::-webkit-scrollbar {
                width: 6px;
            }
            
            #chat-input::-webkit-scrollbar-track {
                background: transparent;
            }
            
            #chat-input::-webkit-scrollbar-thumb {
                background: #888;
                border-radius: 3px;
            }
            
            #chat-input::-webkit-scrollbar-thumb:hover {
                background: #555;
            }
            
            .element-highlight {
                outline: 3px solid #3B82F6 !important;
                outline-offset: 2px !important;
                background-color: rgba(59, 130, 246, 0.1) !important;
                cursor: crosshair !important;
                position: relative;
            }

            .element-highlight::after {
                content: attr(data-element-info);
                position: absolute;
                top: -25px;
                left: 0;
                background: #3B82F6;
                color: white;
                padding: 2px 6px;
                border-radius: 3px;
                font-size: 11px;
                white-space: nowrap;
                z-index: 10000;
                pointer-events: none;
            }

            .element-selecting * {
                cursor: crosshair !important;
            }

            #chat-messages::-webkit-scrollbar {
                width: 6px;
            }

            #chat-messages::-webkit-scrollbar-track {
                background: rgba(0, 0, 0, 0.1);
                border-radius: 3px;
            }

            #chat-messages::-webkit-scrollbar-thumb {
                background: rgba(0, 0, 0, 0.3);
                border-radius: 3px;
            }

            #chat-messages::-webkit-scrollbar-thumb:hover {
                background: rgba(0, 0, 0, 0.5);
            }

            .chat-message {
                animation: fadeIn 0.3s ease-out;
            }

            @keyframes fadeIn {
                from { opacity: 0; transform: translateY(10px); }
                to { opacity: 1; transform: translateY(0); }
            }
        `;
        document.head.appendChild(styles);

        // Add to page
        document.body.appendChild(chatContainer);
    }

    function attachEventListeners() {
        // Toggle chat
        document.getElementById('claude-chat-button').addEventListener('click', toggleChat);
        document.getElementById('chat-minimize-btn').addEventListener('click', toggleChat);

        // Send message
        document.getElementById('chat-send-btn').addEventListener('click', sendMessage);
        
        const chatInput = document.getElementById('chat-input');
        
        // Handle Enter/Shift+Enter
        chatInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                sendMessage();
            }
        });
        
        // Auto-resize textarea as user types
        chatInput.addEventListener('input', function() {
            autoResizeTextarea(this);
        });

        // Context toggle
        document.getElementById('chat-context-btn').addEventListener('click', toggleContext);

        // Element selection
        document.getElementById('chat-select-element').addEventListener('click', startElementSelection);

        // Track mouse position
        document.addEventListener('mousemove', (e) => {
            chatState.context.mousePosition = { x: e.clientX, y: e.clientY };
        });

        // Track page visibility
        document.addEventListener('visibilitychange', () => {
            chatState.context.pageVisible = !document.hidden;
        });
    }

    function toggleChat() {
        chatState.isOpen = !chatState.isOpen;
        const button = document.getElementById('claude-chat-button');
        const window = document.getElementById('claude-chat-window');

        if (chatState.isOpen) {
            button.style.display = 'none';
            window.classList.remove('hidden');
            document.getElementById('chat-input').focus();
            collectPageContext(); // Refresh context when opening
            connectWebSocket(); // Connect to real-time chat
            
            // Resume ticket polling if we have active tickets
            if (chatState.activeTickets.length > 0) {
                startTicketPolling();
            }
        } else {
            button.style.display = 'block';
            window.classList.add('hidden');
            disconnectWebSocket(); // Disconnect when closing
            stopTicketPolling(); // Stop polling when chat closes
        }
    }
    
    function connectWebSocket() {
        if (chatState.websocket && chatState.websocket.readyState === WebSocket.OPEN) {
            return; // Already connected
        }
        
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws/chat?session=${chatState.sessionId}&page=${encodeURIComponent(chatState.context.page)}`;
        
        try {
            chatState.websocket = new WebSocket(wsUrl);
            
            chatState.websocket.onopen = function() {
                chatState.isConnected = true;
                updateConnectionStatus('Connected');
                console.log('WebSocket connected');
            };
            
            chatState.websocket.onmessage = function(event) {
                try {
                    const message = JSON.parse(event.data);
                    handleIncomingMessage(message);
                } catch (e) {
                    console.error('Failed to parse message:', e);
                }
            };
            
            chatState.websocket.onclose = function() {
                chatState.isConnected = false;
                updateConnectionStatus('Disconnected - Reconnecting...');
                // Try to reconnect after 3 seconds
                setTimeout(() => {
                    if (chatState.isOpen) {
                        connectWebSocket();
                    }
                }, 3000);
            };
            
            chatState.websocket.onerror = function(error) {
                console.error('WebSocket error:', error);
                updateConnectionStatus('Connection error');
            };
        } catch (error) {
            console.error('Failed to connect WebSocket:', error);
            updateConnectionStatus('Failed to connect');
        }
    }
    
    function disconnectWebSocket() {
        if (chatState.websocket) {
            chatState.websocket.close();
            chatState.websocket = null;
            chatState.isConnected = false;
        }
    }
    
    function handleIncomingMessage(message) {
        // Add message to UI
        if (message.type === 'claude') {
            addMessage('claude', message.message);
            // Play notification sound if available
            playNotificationSound();
        } else if (message.type === 'system') {
            addMessage('system', message.message);
        } else if (message.type === 'user' && message.sessionId !== chatState.sessionId) {
            // Message from another session (if multiple tabs open)
            addMessage('user', message.message);
        }
        
        // Show typing indicator
        if (message.type === 'typing') {
            showTypingIndicator(message.isTyping);
        }
    }
    
    function updateConnectionStatus(status) {
        const statusEl = document.getElementById('chat-status');
        if (statusEl) {
            statusEl.textContent = status;
            statusEl.className = chatState.isConnected ? 'text-green-500' : 'text-red-500';
        }
    }
    
    function playNotificationSound() {
        // Optional: Add a subtle notification sound
        try {
            const audio = new Audio('data:audio/wav;base64,UklGRnoGAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQoGAACBhYqFbF1fdJivrJBhNjVgodDbq2EcBj+a2/LDciUFLIHO8tiJNwgZaLvt559NEAxQp+PwtmMcBjiR1/LMeSwFJHfH8N2QQAoUXrTp66hVFApGn+DyvmwhBTGH0fPTgjMGHm7A7+OZURE');
            audio.volume = 0.1;
            audio.play();
        } catch (e) {
            // Ignore audio errors
        }
    }
    
    function showTypingIndicator(show) {
        // TODO: Add typing indicator UI
    }
    
    function startTicketPolling() {
        // Only poll if we have active tickets and not already polling
        if (chatState.activeTickets.length === 0 || chatState.ticketPollInterval) {
            return;
        }
        
        // Poll every 30 seconds for ticket updates
        chatState.ticketPollInterval = setInterval(() => {
            checkTicketUpdates();
        }, 30000);
        
        // Do an immediate check
        checkTicketUpdates();
    }
    
    function stopTicketPolling() {
        if (chatState.ticketPollInterval) {
            clearInterval(chatState.ticketPollInterval);
            chatState.ticketPollInterval = null;
        }
    }
    
    function checkTicketUpdates() {
        // Get list of ticket numbers to check
        const ticketNumbers = chatState.activeTickets
            .filter(t => t.status !== 'closed')
            .map(t => t.number);
        
        if (ticketNumbers.length === 0) {
            stopTicketPolling();
            return;
        }
        
        // Check ticket status
        apiFetch('/api/claude/tickets/status', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            },
            body: JSON.stringify({ tickets: ticketNumbers })
        })
        .then(response => response.json())
        .then(data => {
            if (data.success && data.tickets) {
                data.tickets.forEach(ticketUpdate => {
                    const ticket = chatState.activeTickets.find(t => t.number === ticketUpdate.number);
                    if (ticket) {
                        // Check if status changed
                        if (ticket.status !== ticketUpdate.status) {
                            handleTicketStatusChange(ticket, ticketUpdate);
                        }
                        
                        // Check for new responses
                        if (ticketUpdate.newResponses && ticketUpdate.newResponses.length > 0) {
                            ticketUpdate.newResponses.forEach(response => {
                                addMessage('claude', `Update on ticket #${ticket.number}: ${response.message}`);
                            });
                        }
                        
                        // Update ticket info
                        ticket.status = ticketUpdate.status;
                        ticket.lastChecked = new Date().toISOString();
                    }
                });
            }
        })
        .catch(error => {
            console.error('Failed to check ticket updates:', error);
        });
    }
    
    function handleTicketStatusChange(ticket, update) {
        const statusMessages = {
            'open': `Ticket #${ticket.number} is now being worked on by Claude.`,
            'pending': `Ticket #${ticket.number} is waiting for additional information.`,
            'closed': `Ticket #${ticket.number} has been resolved! ${update.resolution || ''}`,
            'merged': `Ticket #${ticket.number} was merged with another ticket.`
        };
        
        const message = statusMessages[update.status] || `Ticket #${ticket.number} status changed to ${update.status}.`;
        addMessage('system', message);
        
        // Play notification sound for important updates
        if (update.status === 'closed' || update.status === 'open') {
            playNotificationSound();
        }
    }

    function toggleContext() {
        const contextBar = document.getElementById('chat-context-bar');
        contextBar.classList.toggle('hidden');
    }
    
    function autoResizeTextarea(textarea) {
        // Reset height to calculate new height
        textarea.style.height = '40px';
        
        // Calculate new height based on content
        const scrollHeight = textarea.scrollHeight;
        const maxHeight = 120; // Max height in pixels
        
        // Set new height, capped at maxHeight
        if (scrollHeight > maxHeight) {
            textarea.style.height = maxHeight + 'px';
            textarea.style.overflowY = 'auto';
        } else {
            textarea.style.height = scrollHeight + 'px';
            textarea.style.overflowY = 'hidden';
        }
    }

    function startElementSelection() {
        document.body.classList.add('element-selecting');
        const statusText = document.getElementById('chat-status');
        statusText.textContent = 'Click on an element to select it...';

        // Highlight elements on hover
        let lastHighlighted = null;
        
        const mouseMoveHandler = (e) => {
            if (lastHighlighted && lastHighlighted !== e.target) {
                lastHighlighted.classList.remove('element-highlight');
            }
            
            // Skip the chat container itself
            if (!e.target.closest('#claude-chat-container')) {
                e.target.classList.add('element-highlight');
                const selector = generateSelector(e.target);
                e.target.setAttribute('data-element-info', selector);
                lastHighlighted = e.target;
            }
        };

        const clickHandler = (e) => {
            e.preventDefault();
            e.stopPropagation();
            
            // Skip if clicking on chat
            if (e.target.closest('#claude-chat-container')) {
                return;
            }

            // Clean up
            document.body.classList.remove('element-selecting');
            if (lastHighlighted) {
                lastHighlighted.classList.remove('element-highlight');
            }
            document.removeEventListener('mousemove', mouseMoveHandler);
            document.removeEventListener('click', clickHandler, true);

            // Store selected element info
            const element = e.target;
            const selector = generateSelector(element);
            const rect = element.getBoundingClientRect();
            
            chatState.context.selectedElement = {
                selector: selector,
                tagName: element.tagName,
                id: element.id,
                className: element.className,
                text: element.textContent.substring(0, 100),
                position: {
                    top: rect.top,
                    left: rect.left,
                    width: rect.width,
                    height: rect.height
                },
                attributes: Array.from(element.attributes).map(attr => ({
                    name: attr.name,
                    value: attr.value
                }))
            };

            // Update UI
            const infoEl = document.getElementById('selected-element-info');
            infoEl.classList.remove('hidden');
            infoEl.querySelector('span').textContent = selector;
            statusText.textContent = 'Element selected';

            // Add context message
            addMessage('system', `Selected element: ${selector}`);
        };

        document.addEventListener('mousemove', mouseMoveHandler);
        document.addEventListener('click', clickHandler, true);
    }

    function generateSelector(element) {
        if (element.id) {
            return `#${element.id}`;
        }
        
        let path = [];
        while (element && element.nodeType === Node.ELEMENT_NODE) {
            let selector = element.nodeName.toLowerCase();
            
            if (element.className && typeof element.className === 'string') {
                const classes = element.className.split(/\s+/).filter(c => c && !c.includes('element-highlight'));
                if (classes.length > 0) {
                    selector += '.' + classes.slice(0, 2).join('.');
                }
            }
            
            path.unshift(selector);
            if (element.id) break;
            element = element.parentNode;
        }
        
        return path.join(' > ');
    }

    function collectPageContext() {
        // Always update current location
        chatState.context.page = window.location.pathname;
        chatState.context.url = window.location.href;
        chatState.context.timestamp = new Date().toISOString();
        
        // Collect form data if any
        const forms = document.querySelectorAll('form');
        chatState.context.forms = Array.from(forms).map(form => ({
            id: form.id,
            action: form.action,
            method: form.method,
            fields: Array.from(form.elements).map(el => ({
                name: el.name,
                type: el.type,
                value: el.type === 'password' ? '[hidden]' : el.value
            }))
        }));

        // Collect error messages if visible
        chatState.context.errors = Array.from(
            document.querySelectorAll('.error, .alert-danger, [role="alert"]')
        ).map(el => el.textContent.trim());

        // Get user info if available
        const userElement = document.querySelector('[data-user-id], .user-name, #current-user');
        if (userElement) {
            chatState.context.user = userElement.textContent || userElement.dataset.userId;
        }

        // Page title and meta
        chatState.context.pageTitle = document.title;
        chatState.context.metaDescription = document.querySelector('meta[name="description"]')?.content;

        // Tables on page
        chatState.context.tables = Array.from(document.querySelectorAll('table')).map(table => ({
            id: table.id,
            rows: table.rows.length,
            columns: table.rows[0]?.cells.length || 0
        }));
    }

    function sendMessage() {
        const input = document.getElementById('chat-input');
        const message = input.value.trim();
        
        if (!message) return;

        // Add user message
        addMessage('user', message);
        
        // Clear input and reset height
        input.value = '';
        input.style.height = '40px';
        
        // Refresh context with current page info
        collectPageContext();
        
        // Always include at minimum the current URL and page
        const messageContext = {
            ...chatState.context,
            currentUrl: window.location.href,
            currentPath: window.location.pathname,
            pageTitle: document.title,
            timestamp: new Date().toISOString()
        };
        
        // Check if this is an error report - always use HTTP for ticket creation
        const isErrorReport = /\b(error|500|404|broken|bug|issue|fail|crash|not working)\b/i.test(message);
        
        if (isErrorReport) {
            // Always use HTTP for error reports to ensure ticket creation
            fallbackToHTTP(message);
        } else if (chatState.websocket && chatState.websocket.readyState === WebSocket.OPEN) {
            // Send via WebSocket for real-time chat (non-error messages)
            const wsMessage = {
                type: 'user',
                message: message,
                context: messageContext,
                timestamp: new Date().toISOString(),
                sessionId: chatState.sessionId
            };
            
            try {
                chatState.websocket.send(JSON.stringify(wsMessage));
                updateConnectionStatus('Connected');
            } catch (error) {
                console.error('Failed to send WebSocket message:', error);
                fallbackToHTTP(message);
            }
        } else {
            // Fallback to HTTP if WebSocket not connected
            fallbackToHTTP(message);
        }
    }
    
    function fallbackToHTTP(message) {
        // Collect current context
        collectPageContext();
        
        // Always include at minimum the current URL and page
        const messageContext = {
            ...chatState.context,
            currentUrl: window.location.href,
            currentPath: window.location.pathname,
            pageTitle: document.title,
            timestamp: new Date().toISOString()
        };
        
        // Prepare payload
        const payload = {
            message: message,
            context: messageContext,
            timestamp: new Date().toISOString()
        };

        // Update status
        const statusEl = document.getElementById('chat-status');
        statusEl.textContent = 'Sending via HTTP...';

        // Send to backend
        apiFetch('/api/claude-feedback', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'X-Requested-With': 'XMLHttpRequest'
            },
            body: JSON.stringify(payload)
        })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                // Check if a ticket was created
                if (data.ticketNumber) {
                    chatState.activeTickets.push({
                        number: data.ticketNumber,
                        created: new Date().toISOString(),
                        lastChecked: null,
                        status: 'new'
                    });
                    
                    // Use the response from the API which includes the ticket number and context
                    addMessage('claude', data.response || `Ticket #${data.ticketNumber} created. I'll work on this issue and update you with progress.`);
                    
                    // Start polling for ticket updates
                    startTicketPolling();
                } else {
                    addMessage('claude', data.response || 'Thanks for your feedback!');
                }
                statusEl.textContent = 'Sent';
            } else {
                addMessage('system', 'Failed to send feedback. The message has been logged to console.');
                console.log('Claude Code Feedback:', payload);
                statusEl.textContent = 'Error';
            }
        })
        .catch(error => {
            // Log to console as fallback
            console.log('Claude Code Feedback:', payload);
            addMessage('system', 'Feedback logged to console (backend not connected)');
            statusEl.textContent = 'Logged to console';
        })
        .finally(() => {
            setTimeout(() => {
                statusEl.textContent = chatState.isConnected ? 'Connected' : 'Offline mode';
            }, 2000);
        });
    }

    function addMessage(type, content) {
        const messagesEl = document.getElementById('chat-messages');
        const messageDiv = document.createElement('div');
        messageDiv.className = 'chat-message';
        
        if (type === 'user') {
            messageDiv.innerHTML = `
                <div class="flex justify-end">
                    <div class="bg-gotrs-600 text-white px-3 py-2 rounded-lg max-w-xs">
                        ${escapeHtml(content)}
                    </div>
                </div>
            `;
        } else if (type === 'claude') {
            messageDiv.innerHTML = `
                <div class="flex justify-start">
                    <div class="bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 px-3 py-2 rounded-lg max-w-xs">
                        ${escapeHtml(content)}
                    </div>
                </div>
            `;
        } else if (type === 'system') {
            messageDiv.innerHTML = `
                <div class="text-center">
                    <span class="text-xs text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                        ${escapeHtml(content)}
                    </span>
                </div>
            `;
        }
        
        messagesEl.appendChild(messageDiv);
        messagesEl.scrollTop = messagesEl.scrollHeight;
        
        // Store message
        chatState.messages.push({ type, content, timestamp: new Date().toISOString() });
        saveChatHistory();
    }

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    function saveChatHistory() {
        try {
            localStorage.setItem('claude-chat-history', JSON.stringify(chatState.messages.slice(-50)));
        } catch (e) {
            console.error('Failed to save chat history:', e);
        }
    }

    function loadChatHistory() {
        try {
            const history = localStorage.getItem('claude-chat-history');
            if (history) {
                chatState.messages = JSON.parse(history);
                // Don't restore messages to UI on page load, just keep in memory
            }
        } catch (e) {
            console.error('Failed to load chat history:', e);
        }
    }

    // Keyboard shortcut (Ctrl/Cmd + Shift + C)
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'C') {
            e.preventDefault();
            toggleChat();
        }
    });

    // Export for debugging
    window.claudeChat = {
        state: chatState,
        toggleChat,
        sendMessage,
        collectPageContext,
        startElementSelection
    };
})();