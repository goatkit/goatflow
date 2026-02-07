# 🌐 GoatFlow API Documentation

Generated from YAML route definitions

## 📋 Route Groups


### Admin: admin-customer-companies

**Description:** Customer company management endpoints  
**Prefix:** `/admin`  
**Middleware:** `auth` `admin` `audit` 


#### List Customer Companies

- **Path:** `/customer-companies`
- **Method:** `GET`
- **Description:** Get list of all customer companies

- **Permissions:** `customer:read` 


- **Parameters:**
  - `limit` (int): 
  - `page` (int): 
  - `search` (string): Search term for company name or ID
  - `valid` (string): Filter by validity status




---

#### Create Customer Company

- **Path:** `/customer-companies/new`
- **Method:** `[GET POST]`
- **Description:** Create a new customer company

- **Permissions:** `customer:write` 




---

#### Edit Customer Company

- **Path:** `/customer-companies/:id/edit`
- **Method:** `[GET POST]`
- **Description:** Edit customer company details

- **Permissions:** `customer:write` 


- **Parameters:**
  - `id` (string)*: Customer company ID
  - `tab` (string): 




---

#### Delete Customer Company

- **Path:** `/customer-companies/:id/delete`
- **Method:** `POST`
- **Description:** Soft delete (deactivate) a customer company

- **Permissions:** `customer:delete` 




---

#### List Company Users

- **Path:** `/customer-companies/:id/users`
- **Method:** `GET`
- **Description:** Get all users belonging to a company

- **Permissions:** `customer:read` 




---

#### List Company Tickets

- **Path:** `/customer-companies/:id/tickets`
- **Method:** `GET`
- **Description:** Get all tickets for a company

- **Permissions:** `customer:read` `ticket:read` 




---

#### Manage Company Services

- **Path:** `/customer-companies/:id/services`
- **Method:** `[GET POST]`
- **Description:** Manage services available to a company

- **Permissions:** `customer:write` `service:assign` 




---

#### Portal Customization

- **Path:** `/customer-companies/:id/portal`
- **Method:** `[GET POST]`
- **Description:** Customize the customer portal for a company

- **Permissions:** `portal:admin` 




---

#### Upload Portal Logo

- **Path:** `/customer-companies/:id/portal/logo`
- **Method:** `POST`
- **Description:** Upload a custom logo for the customer portal

- **Permissions:** `portal:admin` 



- **Rate Limit:** 10 requests per 1h


---



### Agent: agent-dashboard

**Description:** Agent dashboard and ticket management  
**Prefix:** `/agent`  
**Middleware:** `auth` `agent` 


#### Agent Dashboard

- **Path:** `/dashboard`
- **Method:** `GET`
- **Description:** Main dashboard for agents with ticket statistics




---

#### List Tickets

- **Path:** `/tickets`
- **Method:** `GET`
- **Description:** List tickets with advanced filtering


- **Parameters:**
  - `assigned_to` (string): Filter by assigned agent
  - `limit` (int): 
  - `page` (int): 
  - `priority` (int): 
  - `queue` (int): Filter by queue ID
  - `search` (string): Search in ticket content
  - `state` (string): 




---

#### View Ticket

- **Path:** `/tickets/:id`
- **Method:** `GET`
- **Description:** View detailed ticket information


- **Parameters:**
  - `id` (int)*: 




---

#### Update Ticket

- **Path:** `/tickets/:id`
- **Method:** `PUT`
- **Description:** Update ticket properties

- **Permissions:** `ticket:write` 




---

#### Add Note

- **Path:** `/tickets/:id/note`
- **Method:** `POST`
- **Description:** Add an internal note to a ticket

- **Permissions:** `ticket:note` 




---

#### Reply to Ticket

- **Path:** `/tickets/:id/reply`
- **Method:** `POST`
- **Description:** Send a reply to the customer

- **Permissions:** `ticket:reply` 




---

#### Assign Ticket

- **Path:** `/tickets/:id/assign`
- **Method:** `POST`
- **Description:** Assign ticket to an agent

- **Permissions:** `ticket:assign` 




---

#### Merge Tickets

- **Path:** `/tickets/:id/merge`
- **Method:** `POST`
- **Description:** Merge multiple tickets into one

- **Permissions:** `ticket:merge` 




---

#### Split Ticket

- **Path:** `/tickets/:id/split`
- **Method:** `POST`
- **Description:** Split a ticket into multiple tickets

- **Permissions:** `ticket:split` 




---

#### List Customers

- **Path:** `/customers`
- **Method:** `GET`
- **Description:** List and search customers

- **Permissions:** `customer:read` 




---

#### View Customer

- **Path:** `/customers/:id`
- **Method:** `GET`
- **Description:** View customer details and ticket history

- **Permissions:** `customer:read` 




---

#### Customer Tickets

- **Path:** `/customers/:id/tickets`
- **Method:** `GET`
- **Description:** Get all tickets for a specific customer

- **Permissions:** `customer:read` `ticket:read` 




---

#### List Queues

- **Path:** `/queues`
- **Method:** `GET`
- **Description:** List available queues for the agent




---

#### Queue Tickets

- **Path:** `/queues/:id/tickets`
- **Method:** `GET`
- **Description:** Get all tickets in a specific queue


- **Parameters:**
  - `id` (int)*: 




---

#### Agent Statistics

- **Path:** `/stats`
- **Method:** `GET`
- **Description:** Get agent performance statistics




---

#### Ticket Statistics

- **Path:** `/stats/tickets`
- **Method:** `GET`
- **Description:** Get ticket statistics for the agent


- **Parameters:**
  - `period` (string): 




---

#### Global Search

- **Path:** `/search`
- **Method:** `GET`
- **Description:** Search across tickets, customers, and articles


- **Parameters:**
  - `limit` (int): 
  - `q` (string)*: 
  - `type` (string): 




---



### Core: authentication

**Description:** Authentication and authorization endpoints  
**Prefix:** ``  
**Middleware:** 


#### User Login

- **Path:** `/login`
- **Method:** `[GET POST]`
- **Description:** Login endpoint for agents and customers



- **Rate Limit:** 10 requests per 1m


---

#### User Logout

- **Path:** `/logout`
- **Method:** `POST`
- **Description:** Logout and invalidate session




---

#### Refresh Token

- **Path:** `/auth/refresh`
- **Method:** `POST`
- **Description:** Refresh authentication token




---

#### Verify Authentication

- **Path:** `/auth/verify`
- **Method:** `GET`
- **Description:** Verify if current session is valid




---

#### Password Reset

- **Path:** `/auth/password/reset`
- **Method:** `[GET POST]`
- **Description:** Request password reset



- **Rate Limit:** 3 requests per 1h


---

#### Change Password

- **Path:** `/auth/password/change`
- **Method:** `POST`
- **Description:** Change user password




---



### Core: health-checks

**Description:** Health check and monitoring endpoints  
**Prefix:** ``  
**Middleware:** 


#### Health Check

- **Path:** `/health`
- **Method:** `GET`
- **Description:** Basic health check endpoint




---

#### Detailed Health Check

- **Path:** `/health/detailed`
- **Method:** `GET`
- **Description:** Detailed health check with component status

- **Permissions:** `monitoring:read` 




---

#### Prometheus Metrics

- **Path:** `/metrics`
- **Method:** `GET`
- **Description:** Expose metrics for Prometheus scraping




---



### Customer: customer-portal

**Description:** Customer portal endpoints  
**Prefix:** `/customer`  
**Middleware:** `auth` `customer` 


#### Customer Dashboard

- **Path:** `/dashboard`
- **Method:** `GET`
- **Description:** Customer portal main dashboard




---

#### List Customer Tickets

- **Path:** `/tickets`
- **Method:** `GET`
- **Description:** List all tickets for the customer


- **Parameters:**
  - `order` (string): 
  - `search` (string): Search in ticket number or title
  - `sort` (string): 
  - `status` (string): 




---

#### Create Ticket

- **Path:** `/tickets/new`
- **Method:** `[GET POST]`
- **Description:** Create a new support ticket



- **Rate Limit:** 10 requests per 1h


---

#### View Ticket

- **Path:** `/tickets/:id`
- **Method:** `GET`
- **Description:** View ticket details and conversation


- **Parameters:**
  - `id` (int)*: Ticket ID




---

#### Reply to Ticket

- **Path:** `/tickets/:id/reply`
- **Method:** `POST`
- **Description:** Add a reply to an existing ticket



- **Rate Limit:** 20 requests per 1h


---

#### Close Ticket

- **Path:** `/tickets/:id/close`
- **Method:** `POST`
- **Description:** Close a ticket




---

#### Customer Profile

- **Path:** `/profile`
- **Method:** `[GET POST]`
- **Description:** View and update customer profile




---

#### Change Password

- **Path:** `/profile/password`
- **Method:** `[GET POST]`
- **Description:** Change customer password



- **Rate Limit:** 5 requests per 1h


---

#### Knowledge Base

- **Path:** `/kb`
- **Method:** `GET`
- **Description:** Browse knowledge base articles




---

#### Search Knowledge Base

- **Path:** `/kb/search`
- **Method:** `GET`
- **Description:** Search knowledge base articles


- **Parameters:**
  - `q` (string)*: Search query




---

#### View KB Article

- **Path:** `/kb/article/:id`
- **Method:** `GET`
- **Description:** View a knowledge base article




---

#### Company Information

- **Path:** `/company`
- **Method:** `GET`
- **Description:** View company information




---

#### Company Users

- **Path:** `/company/users`
- **Method:** `GET`
- **Description:** View other users in the same company

- **Permissions:** `company:users:read` 




---




---
*Generated by GoatFlow Route Documentation Generator*
