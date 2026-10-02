import { HttpClient } from '../client.js';
import {
  Article,
  ArticleCreateRequest,
  ArticleList,
  ArticleUpdate,
  ArticleUpdateRequest,
  CreatedTicket,
  ReopenResult,
  Ticket,
  TicketCreateRequest,
  TicketList,
  TicketListOptions,
  TicketRecord,
  TicketSummary,
  TicketUpdateRequest,
  GoatflowError,
} from '../types.js';

/** /api/v1/tickets */
export class TicketsService {
  constructor(private readonly client: HttpClient) {}

  /** One page of tickets the caller can read. */
  async list(options: TicketListOptions = {}): Promise<TicketList> {
    const { data, pagination } = await this.client.request<TicketSummary[]>('GET', '/api/v1/tickets', {
      query: { ...options },
    });
    if (!pagination) {
      throw new GoatflowError('Ticket list response has no pagination');
    }
    return { tickets: data, pagination };
  }

  async get(id: number): Promise<Ticket> {
    return this.client.get<Ticket>(`/api/v1/tickets/${id}`);
  }

  async create(data: TicketCreateRequest): Promise<CreatedTicket> {
    return this.client.post<CreatedTicket>('/api/v1/tickets', data);
  }

  /** Changes the given fields and returns the updated row. */
  async update(id: number, data: TicketUpdateRequest): Promise<TicketRecord> {
    return this.client.put<TicketRecord>(`/api/v1/tickets/${id}`, data);
  }

  async delete(id: number): Promise<void> {
    await this.client.delete(`/api/v1/tickets/${id}`);
  }

  /** Moves a closed ticket back to "open" and records the reason as an article. */
  async reopen(id: number, reason: string): Promise<ReopenResult> {
    return this.client.post<ReopenResult>(`/api/v1/tickets/${id}/reopen`, { reason });
  }
}

/** /api/v1/tickets/:id/articles */
export class ArticlesService {
  constructor(private readonly client: HttpClient) {}

  /** A ticket's articles, newest first. */
  async list(ticketId: number, includeAttachments = false): Promise<ArticleList> {
    return this.client.get<ArticleList>(`/api/v1/tickets/${ticketId}/articles`, {
      include_attachments: includeAttachments,
    });
  }

  async get(ticketId: number, articleId: number, includeAttachments = false): Promise<Article> {
    return this.client.get<Article>(`/api/v1/tickets/${ticketId}/articles/${articleId}`, {
      include_attachments: includeAttachments,
    });
  }

  async create(ticketId: number, data: ArticleCreateRequest): Promise<Article> {
    return this.client.post<Article>(`/api/v1/tickets/${ticketId}/articles`, data);
  }

  async update(ticketId: number, articleId: number, data: ArticleUpdateRequest): Promise<ArticleUpdate> {
    return this.client.put<ArticleUpdate>(`/api/v1/tickets/${ticketId}/articles/${articleId}`, data);
  }

  /** Deletes an article and its attachments. */
  async delete(ticketId: number, articleId: number): Promise<void> {
    await this.client.delete(`/api/v1/tickets/${ticketId}/articles/${articleId}`);
  }
}
