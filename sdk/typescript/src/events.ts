import WebSocket from 'ws';
import { WebSocketEvent, TicketEvent, MessageEvent, GoatflowError } from './types';

export interface EventSubscription {
  unsubscribe(): void;
}

export type EventHandler<T = any> = (event: T) => void;

/**
 * Real-time event client for GoatFlow WebSocket connections
 */
export class EventsClient {
  private ws?: WebSocket;
  private url: string;
  private auth?: string;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private listeners = new Map<string, Set<EventHandler>>();
  private isConnecting = false;
  private isConnected = false;

  constructor(baseURL: string, auth?: string) {
    // Convert HTTP URL to WebSocket URL
    this.url = baseURL.replace(/^https?:/, 'wss:').replace(/^http:/, 'ws:') + '/api/v1/events';
    this.auth = auth;
  }

  /**
   * Connect to the WebSocket server
   */
  async connect(): Promise<void> {
    if (this.isConnecting || this.isConnected) {
      return;
    }

    this.isConnecting = true;

    return new Promise((resolve, reject) => {
      try {
        const headers: Record<string, string> = {};
        if (this.auth) {
          headers['Authorization'] = this.auth;
        }

        this.ws = new WebSocket(this.url, { headers });

        this.ws.on('open', () => {
          this.isConnecting = false;
          this.isConnected = true;
          this.reconnectAttempts = 0;
          this.emit('connected', {});
          resolve();
        });

        this.ws.on('message', (data: WebSocket.Data) => {
          try {
            const event = JSON.parse(data.toString()) as WebSocketEvent;
            this.handleEvent(event);
          } catch (error) {
            console.error('Failed to parse WebSocket message:', error);
          }
        });

        this.ws.on('close', (code: number, reason: string) => {
          this.isConnecting = false;
          this.isConnected = false;
          this.emit('disconnected', { code, reason });

          // Attempt to reconnect unless intentionally closed
          if (code !== 1000 && this.reconnectAttempts < this.maxReconnectAttempts) {
            this.scheduleReconnect();
          }
        });

        this.ws.on('error', (error: Error) => {
          this.isConnecting = false;
          this.isConnected = false;
          this.emit('error', error);
          reject(new GoatflowError('WebSocket connection failed', undefined, 'WEBSOCKET_ERROR', error.message));
        });

        // Connection timeout
        setTimeout(() => {
          if (this.isConnecting) {
            this.isConnecting = false;
            if (this.ws) {
              this.ws.close();
            }
            reject(new GoatflowError('WebSocket connection timeout', undefined, 'WEBSOCKET_TIMEOUT'));
          }
        }, 10000);

      } catch (error) {
        this.isConnecting = false;
        reject(error);
      }
    });
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.close(1000, 'Client disconnect');
      this.ws = undefined;
    }
    this.isConnected = false;
    this.isConnecting = false;
  }

  /**
   * Schedule a reconnection attempt
   */
  private scheduleReconnect(): void {
    this.reconnectAttempts++;
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1);

    setTimeout(() => {
      if (!this.isConnected && !this.isConnecting) {
        this.connect().catch(error => {
          console.error('Reconnection failed:', error);
        });
      }
    }, delay);
  }

  /**
   * Handle incoming WebSocket events
   */
  private handleEvent(event: WebSocketEvent): void {
    this.emit(event.type, event);
    this.emit('*', event); // Wildcard listeners
  }

  /**
   * Emit an event to all listeners
   */
  private emit(eventType: string, event: any): void {
    const listeners = this.listeners.get(eventType);
    if (listeners) {
      listeners.forEach(handler => {
        try {
          handler(event);
        } catch (error) {
          console.error('Error in event handler:', error);
        }
      });
    }
  }

  /**
   * Subscribe to a specific event type
   */
  on<T = WebSocketEvent>(eventType: string, handler: EventHandler<T>): EventSubscription {
    if (!this.listeners.has(eventType)) {
      this.listeners.set(eventType, new Set());
    }
    
    const listeners = this.listeners.get(eventType)!;
    listeners.add(handler as EventHandler);

    return {
      unsubscribe: () => {
        listeners.delete(handler as EventHandler);
        if (listeners.size === 0) {
          this.listeners.delete(eventType);
        }
      }
    };
  }

  /**
   * Subscribe to all events
   */
  onAny(handler: EventHandler<WebSocketEvent>): EventSubscription {
    return this.on('*', handler);
  }

  /**
   * Subscribe to ticket events
   */
  onTicket(handler: EventHandler<TicketEvent>): EventSubscription {
    const subscription = this.on('ticket.*', handler);
    
    // Also subscribe to specific ticket events
    const subscriptions = [
      this.on('ticket.created', handler),
      this.on('ticket.updated', handler),
      this.on('ticket.closed', handler),
      this.on('ticket.assigned', handler),
    ];

    return {
      unsubscribe: () => {
        subscription.unsubscribe();
        subscriptions.forEach(sub => sub.unsubscribe());
      }
    };
  }

  /**
   * Subscribe to message events
   */
  onMessage(handler: EventHandler<MessageEvent>): EventSubscription {
    const subscriptions = [
      this.on('message.created', handler),
      this.on('message.updated', handler),
    ];

    return {
      unsubscribe: () => {
        subscriptions.forEach(sub => sub.unsubscribe());
      }
    };
  }

  /**
   * Subscribe to connection events
   */
  onConnection(handlers: {
    connected?: () => void;
    disconnected?: (event: { code: number; reason: string }) => void;
    error?: (error: Error) => void;
  }): EventSubscription {
    const subscriptions: EventSubscription[] = [];

    if (handlers.connected) {
      subscriptions.push(this.on('connected', handlers.connected));
    }
    if (handlers.disconnected) {
      subscriptions.push(this.on('disconnected', handlers.disconnected));
    }
    if (handlers.error) {
      subscriptions.push(this.on('error', handlers.error));
    }

    return {
      unsubscribe: () => {
        subscriptions.forEach(sub => sub.unsubscribe());
      }
    };
  }

  /**
   * Remove all listeners for a specific event type
   */
  off(eventType: string): void {
    this.listeners.delete(eventType);
  }

  /**
   * Remove all listeners
   */
  removeAllListeners(): void {
    this.listeners.clear();
  }

  /**
   * Check if the client is connected
   */
  get connected(): boolean {
    return this.isConnected;
  }

  /**
   * Check if the client is connecting
   */
  get connecting(): boolean {
    return this.isConnecting;
  }

  /**
   * Get the current WebSocket ready state
   */
  get readyState(): number {
    return this.ws ? this.ws.readyState : WebSocket.CLOSED;
  }

  /**
   * Send a message to the server (if supported)
   */
  send(message: any): void {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new GoatflowError('WebSocket is not connected', undefined, 'WEBSOCKET_NOT_CONNECTED');
    }

    this.ws.send(JSON.stringify(message));
  }

  /**
   * Set authentication token
   */
  setAuth(auth: string): void {
    this.auth = auth;
    
    // If connected, reconnect with new auth
    if (this.isConnected) {
      this.disconnect();
      this.connect().catch(error => {
        console.error('Failed to reconnect with new auth:', error);
      });
    }
  }
}