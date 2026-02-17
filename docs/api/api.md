# 🌐 GoatFlow API Documentation

Generated from YAML route definitions

## 📋 Route Groups


### Default: admin

**Description:** Administrative routes for system management  
**Prefix:** `/admin`  
**Middleware:** `auth` `admin` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Display admin dashboard with system overview




---

#### 

- **Path:** `/users`
- **Method:** `GET`
- **Description:** Display user management page




---

#### 

- **Path:** `/users/:id`
- **Method:** `GET`
- **Description:** Display user details




---

#### 

- **Path:** `/users/:id/edit`
- **Method:** `GET`
- **Description:** Display user edit form




---

#### 

- **Path:** `/users/:id`
- **Method:** `PUT`
- **Description:** Update user




---

#### 

- **Path:** `/users/:id`
- **Method:** `DELETE`
- **Description:** Delete user




---

#### 

- **Path:** `/users`
- **Method:** `POST`
- **Description:** Create new user




---

#### 

- **Path:** `/users/:id/groups`
- **Method:** `GET`
- **Description:** List user groups




---

#### 

- **Path:** `/users/:id/status`
- **Method:** ``
- **Description:** Toggle user activation status




---

#### 

- **Path:** `/users/:id/reset-password`
- **Method:** `POST`
- **Description:** Reset user password




---

#### 

- **Path:** `/password-policy`
- **Method:** `GET`
- **Description:** Get password policy configuration




---

#### 

- **Path:** `/users/list`
- **Method:** `GET`
- **Description:** Get all users (JSON API)




---

#### 

- **Path:** `/groups`
- **Method:** `GET`
- **Description:** Display group management page




---

#### 

- **Path:** `/groups/new`
- **Method:** `GET`
- **Description:** Display new group creation form




---

#### 

- **Path:** `/groups`
- **Method:** `POST`
- **Description:** Create new group




---

#### 

- **Path:** `/groups/:id`
- **Method:** `GET`
- **Description:** Display group details




---

#### 

- **Path:** `/groups/:id/edit`
- **Method:** `GET`
- **Description:** Display group edit form




---

#### 

- **Path:** `/groups/:id`
- **Method:** `PUT`
- **Description:** Update existing group




---

#### 

- **Path:** `/groups/:id`
- **Method:** `DELETE`
- **Description:** Delete group




---

#### 

- **Path:** `/groups/:id/members`
- **Method:** `GET`
- **Description:** Display group members




---

#### 

- **Path:** `/groups/:id/members`
- **Method:** `POST`
- **Description:** Add user to group




---

#### 

- **Path:** `/groups/:id/members/:userId`
- **Method:** `DELETE`
- **Description:** Remove user from group




---

#### 

- **Path:** `/groups/:id/permissions`
- **Method:** `GET`
- **Description:** Get queue-centric permissions for a group




---

#### 

- **Path:** `/groups/:id/permissions`
- **Method:** `POST`
- **Description:** Update queue-centric permissions for a group




---

#### 

- **Path:** `/groups/:id/users`
- **Method:** `GET`
- **Description:** Get group users




---

#### 

- **Path:** `/groups/:id/users`
- **Method:** `POST`
- **Description:** Add user to group




---

#### 

- **Path:** `/groups/:id/users/:userId`
- **Method:** `DELETE`
- **Description:** Remove user from group




---

#### 

- **Path:** `/queues`
- **Method:** `GET`
- **Description:** Display queue management page




---

#### 

- **Path:** `/email-identities`
- **Method:** `GET`
- **Description:** Manage system addresses, salutations, and signatures




---

#### 

- **Path:** `/email-queue`
- **Method:** `GET`
- **Description:** Display email queue management page




---

#### 

- **Path:** `/email-queue/retry/:id`
- **Method:** `POST`
- **Description:** Retry sending a specific email from the queue




---

#### 

- **Path:** `/email-queue/delete/:id`
- **Method:** `POST`
- **Description:** Delete a specific email from the queue




---

#### 

- **Path:** `/email-queue/retry-all`
- **Method:** `POST`
- **Description:** Retry all failed emails in the queue




---

#### 

- **Path:** `/priorities`
- **Method:** `GET`
- **Description:** Display priority management page




---

#### 

- **Path:** `/permissions`
- **Method:** `GET`
- **Description:** Display permission management page




---

#### 

- **Path:** `/permissions/user/:userId`
- **Method:** `GET`
- **Description:** Get user permission matrix




---

#### 

- **Path:** `/permissions/user/:userId`
- **Method:** `PUT`
- **Description:** Update user permissions




---

#### 

- **Path:** `/roles`
- **Method:** `GET`
- **Description:** Display role management page




---

#### 

- **Path:** `/roles`
- **Method:** `POST`
- **Description:** Create a new role




---

#### 

- **Path:** `/roles/:id`
- **Method:** `GET`
- **Description:** Get role details




---

#### 

- **Path:** `/roles/:id`
- **Method:** `PUT`
- **Description:** Update an existing role




---

#### 

- **Path:** `/roles/:id`
- **Method:** `DELETE`
- **Description:** Delete a role




---

#### 

- **Path:** `/roles/:id/users`
- **Method:** `GET`
- **Description:** Display users assigned to a role




---

#### 

- **Path:** `/roles/:id/users/search`
- **Method:** `GET`
- **Description:** Search for users to add to a role (scalable typeahead)




---

#### 

- **Path:** `/roles/:id/users`
- **Method:** `POST`
- **Description:** Add a user to a role




---

#### 

- **Path:** `/roles/:id/users/:userId`
- **Method:** `DELETE`
- **Description:** Remove a user from a role




---

#### 

- **Path:** `/roles/:id/permissions`
- **Method:** `GET`
- **Description:** Display and manage role-group permissions




---

#### 

- **Path:** `/roles/:id/permissions`
- **Method:** `POST`
- **Description:** Update role-group permissions




---

#### 

- **Path:** `/states`
- **Method:** `GET`
- **Description:** Display state management page




---

#### 

- **Path:** `/types`
- **Method:** `GET`
- **Description:** Display type management page




---

#### 

- **Path:** `/services`
- **Method:** `GET`
- **Description:** Display service management page




---

#### 

- **Path:** `/services/create`
- **Method:** `POST`
- **Description:** Create a new service




---

#### 

- **Path:** `/services/:id/update`
- **Method:** `PUT`
- **Description:** Update an existing service




---

#### 

- **Path:** `/services/:id/delete`
- **Method:** `DELETE`
- **Description:** Delete a service




---

#### 

- **Path:** `/sla`
- **Method:** `GET`
- **Description:** Display SLA management page




---

#### 

- **Path:** `/sla/create`
- **Method:** `POST`
- **Description:** Create a new SLA




---

#### 

- **Path:** `/sla/:id/update`
- **Method:** `PUT`
- **Description:** Update an existing SLA




---

#### 

- **Path:** `/sla/:id/delete`
- **Method:** `DELETE`
- **Description:** Delete an SLA




---

#### 

- **Path:** `/lookups`
- **Method:** `GET`
- **Description:** Display lookup management page




---

#### 

- **Path:** `/customer/companies`
- **Method:** `GET`
- **Description:** Display customer companies management




---

#### 

- **Path:** `/customer/companies/new`
- **Method:** `GET`
- **Description:** Display new customer company form




---

#### 

- **Path:** `/customer/companies/new`
- **Method:** `POST`
- **Description:** Create new customer company




---

#### 

- **Path:** `/customer/companies`
- **Method:** `POST`
- **Description:** Create new customer company




---

#### 

- **Path:** `/customer/companies/:id/edit`
- **Method:** `GET`
- **Description:** Display customer company edit form




---

#### 

- **Path:** `/customer/companies/:id/edit`
- **Method:** `POST`
- **Description:** Update customer company




---

#### 

- **Path:** `/customer/companies/:id`
- **Method:** `DELETE`
- **Description:** Delete customer company




---

#### 

- **Path:** `/customer/companies/:id/delete`
- **Method:** `POST`
- **Description:** Delete customer company (legacy POST route)




---

#### 

- **Path:** `/customer/companies/:id/activate`
- **Method:** `POST`
- **Description:** Activate customer company




---

#### 

- **Path:** `/customer/companies/:id/users`
- **Method:** `GET`
- **Description:** Display customer company users




---

#### 

- **Path:** `/customer/companies/:id/tickets`
- **Method:** `GET`
- **Description:** Display customer company tickets




---

#### 

- **Path:** `/customer/companies/:id/services`
- **Method:** `GET`
- **Description:** Display customer company services




---

#### 

- **Path:** `/customer/companies/:id/services`
- **Method:** `POST`
- **Description:** Update customer company services




---

#### 

- **Path:** `/customer/companies/:id/services`
- **Method:** `PUT`
- **Description:** Update customer company services




---

#### 

- **Path:** `/customer/companies/:id/portal-settings`
- **Method:** `GET`
- **Description:** Display customer portal settings for a company




---

#### 

- **Path:** `/customer/companies/:id/portal-settings`
- **Method:** `POST`
- **Description:** Update customer portal settings for a company




---

#### 

- **Path:** `/customer/portal/settings`
- **Method:** ``
- **Description:** Display and update global customer portal settings




---

#### 

- **Path:** `/customer/portal/logo/upload`
- **Method:** `POST`
- **Description:** Upload customer portal logo




---

#### 

- **Path:** `/customer-users`
- **Method:** `GET`
- **Description:** Display customer user management




---

#### 

- **Path:** `/customer-users/:id`
- **Method:** `GET`
- **Description:** Get customer user details




---

#### 

- **Path:** `/customer-users`
- **Method:** `POST`
- **Description:** Create customer user




---

#### 

- **Path:** `/customer-users/:id`
- **Method:** `PUT`
- **Description:** Update customer user




---

#### 

- **Path:** `/customer-users/:id`
- **Method:** `DELETE`
- **Description:** Delete customer user




---

#### 

- **Path:** `/customer-users/:id/tickets`
- **Method:** `GET`
- **Description:** Get customer user tickets




---

#### 

- **Path:** `/customer-users/import`
- **Method:** `GET`
- **Description:** Display customer user import form




---

#### 

- **Path:** `/customer-users/import`
- **Method:** `POST`
- **Description:** Import customer users




---

#### 

- **Path:** `/customer-users/export`
- **Method:** `GET`
- **Description:** Export customer users




---

#### 

- **Path:** `/customer-users/bulk-action`
- **Method:** `POST`
- **Description:** Perform customer user bulk action




---

#### 

- **Path:** `/customer-user-services`
- **Method:** `GET`
- **Description:** Customer user ↔ service relations management




---

#### 

- **Path:** `/customer-user-services/customer/:login`
- **Method:** `GET`
- **Description:** Get services for a customer user




---

#### 

- **Path:** `/customer-user-services/customer/:login`
- **Method:** `POST`
- **Description:** Update services for a customer user




---

#### 

- **Path:** `/customer-user-services/service/:id`
- **Method:** `GET`
- **Description:** Get customer users for a service




---

#### 

- **Path:** `/customer-user-services/service/:id`
- **Method:** `POST`
- **Description:** Update customer users for a service




---

#### 

- **Path:** `/customer-user-services/default`
- **Method:** `GET`
- **Description:** Get default service assignments




---

#### 

- **Path:** `/customer-user-services/default`
- **Method:** `POST`
- **Description:** Update default service assignments




---

#### 

- **Path:** `/settings`
- **Method:** `GET`
- **Description:** Display system configuration page




---

#### 

- **Path:** `/reports`
- **Method:** `GET`
- **Description:** Display reports overview




---

#### 

- **Path:** `/backup`
- **Method:** `GET`
- **Description:** Display backup and restore placeholder




---

#### 

- **Path:** `/dynamic-fields`
- **Method:** `GET`
- **Description:** List all dynamic fields




---

#### 

- **Path:** `/dynamic-fields/new`
- **Method:** `GET`
- **Description:** Create new dynamic field form




---

#### 

- **Path:** `/dynamic-fields/screens`
- **Method:** `GET`
- **Description:** Dynamic field screen configuration




---

#### 

- **Path:** `/dynamic-fields/export`
- **Method:** `GET`
- **Description:** Export dynamic fields page




---

#### 

- **Path:** `/dynamic-fields/export`
- **Method:** `POST`
- **Description:** Export selected dynamic fields to YAML




---

#### 

- **Path:** `/dynamic-fields/import`
- **Method:** `GET`
- **Description:** Import dynamic fields page




---

#### 

- **Path:** `/dynamic-fields/import`
- **Method:** `POST`
- **Description:** Upload and preview YAML file for import




---

#### 

- **Path:** `/dynamic-fields/import/confirm`
- **Method:** `POST`
- **Description:** Confirm and execute dynamic fields import




---

#### 

- **Path:** `/dynamic-fields/:id`
- **Method:** `GET`
- **Description:** Edit dynamic field form




---

#### 

- **Path:** `/api/dynamic-fields`
- **Method:** `POST`
- **Description:** Create new dynamic field




---

#### 

- **Path:** `/api/dynamic-fields/:id`
- **Method:** `PUT`
- **Description:** Update existing dynamic field




---

#### 

- **Path:** `/api/dynamic-fields/:id`
- **Method:** `DELETE`
- **Description:** Delete dynamic field




---

#### 

- **Path:** `/api/dynamic-fields/:id/screens`
- **Method:** `PUT`
- **Description:** Save all screen configurations for a field




---

#### 

- **Path:** `/api/dynamic-fields/:id/screen`
- **Method:** `POST`
- **Description:** Toggle single screen configuration for a field




---

#### 

- **Path:** `/templates`
- **Method:** `GET`
- **Description:** Display response templates management page




---

#### 

- **Path:** `/templates/new`
- **Method:** `GET`
- **Description:** Display new template creation form




---

#### 

- **Path:** `/templates/:id/edit`
- **Method:** `GET`
- **Description:** Display template edit form




---

#### 

- **Path:** `/templates/:id/queues`
- **Method:** `GET`
- **Description:** Display template queue assignment page




---

#### 

- **Path:** `/api/templates`
- **Method:** `POST`
- **Description:** Create new response template




---

#### 

- **Path:** `/api/templates/:id`
- **Method:** `PUT`
- **Description:** Update existing response template




---

#### 

- **Path:** `/api/templates/:id`
- **Method:** `DELETE`
- **Description:** Delete response template




---

#### 

- **Path:** `/api/templates/:id/queues`
- **Method:** `PUT`
- **Description:** Update template queue assignments




---

#### 

- **Path:** `/templates/:id/attachments`
- **Method:** `GET`
- **Description:** Display template attachment assignment page




---

#### 

- **Path:** `/api/templates/:id/attachments`
- **Method:** `PUT`
- **Description:** Update template attachment assignments




---

#### 

- **Path:** `/queue-templates`
- **Method:** `GET`
- **Description:** Queue-Template relations overview




---

#### 

- **Path:** `/queues/:id/templates`
- **Method:** `GET`
- **Description:** Assign templates to a queue




---

#### 

- **Path:** `/api/queues/:id/templates`
- **Method:** `PUT`
- **Description:** Update queue template assignments




---

#### 

- **Path:** `/templates/import`
- **Method:** `GET`
- **Description:** Display template import page




---

#### 

- **Path:** `/api/templates/import`
- **Method:** `POST`
- **Description:** Import templates from YAML file




---

#### 

- **Path:** `/api/templates/export`
- **Method:** `GET`
- **Description:** Export all templates as YAML




---

#### 

- **Path:** `/api/templates/:id/export`
- **Method:** `GET`
- **Description:** Export single template as YAML




---

#### 

- **Path:** `/template-attachments`
- **Method:** `GET`
- **Description:** Template-Attachment relations overview




---

#### 

- **Path:** `/attachments/:id/templates`
- **Method:** `GET`
- **Description:** Assign templates to an attachment




---

#### 

- **Path:** `/api/attachments/:id/templates`
- **Method:** `PUT`
- **Description:** Update attachment template assignments




---

#### 

- **Path:** `/attachments`
- **Method:** `GET`
- **Description:** Display standard attachments management page




---

#### 

- **Path:** `/api/attachments`
- **Method:** `POST`
- **Description:** Upload/create a new standard attachment




---

#### 

- **Path:** `/api/attachments/:id`
- **Method:** `PUT`
- **Description:** Update an existing standard attachment




---

#### 

- **Path:** `/api/attachments/:id`
- **Method:** `DELETE`
- **Description:** Delete a standard attachment




---

#### 

- **Path:** `/api/attachments/:id/download`
- **Method:** `GET`
- **Description:** Download a standard attachment file




---

#### 

- **Path:** `/api/attachments/:id/preview`
- **Method:** `GET`
- **Description:** Preview a standard attachment inline




---

#### 

- **Path:** `/modules`
- **Method:** `GET`
- **Description:** List available dynamic modules




---

#### 

- **Path:** `/modules/:module`
- **Method:** ``
- **Description:** Serve dynamic module pages and handle record creation




---

#### 

- **Path:** `/modules/:module/:id`
- **Method:** ``
- **Description:** Handle dynamic module record operations




---

#### 

- **Path:** `/modules/:module/:id/:action`
- **Method:** ``
- **Description:** Execute dynamic module actions




---

#### 

- **Path:** `/signatures`
- **Method:** `GET`
- **Description:** Display email signatures management page




---

#### 

- **Path:** `/signatures/new`
- **Method:** `GET`
- **Description:** Display new signature form




---

#### 

- **Path:** `/signatures/:id`
- **Method:** `GET`
- **Description:** Display edit signature form




---

#### 

- **Path:** `/api/signatures`
- **Method:** `POST`
- **Description:** Create a new signature




---

#### 

- **Path:** `/api/signatures/:id`
- **Method:** `PUT`
- **Description:** Update an existing signature




---

#### 

- **Path:** `/api/signatures/:id`
- **Method:** `DELETE`
- **Description:** Delete a signature




---

#### 

- **Path:** `/signatures/export`
- **Method:** `GET`
- **Description:** Export all signatures as YAML




---

#### 

- **Path:** `/signatures/:id/export`
- **Method:** `GET`
- **Description:** Export single signature as YAML




---

#### 

- **Path:** `/api/signatures/import`
- **Method:** `POST`
- **Description:** Import signatures from YAML file




---

#### 

- **Path:** `/postmaster-filters`
- **Method:** `GET`
- **Description:** Display postmaster filters management page




---

#### 

- **Path:** `/postmaster-filters/new`
- **Method:** `GET`
- **Description:** Display new postmaster filter form




---

#### 

- **Path:** `/postmaster-filters/:name`
- **Method:** `GET`
- **Description:** Display postmaster filter edit form




---

#### 

- **Path:** `/api/postmaster-filters`
- **Method:** `POST`
- **Description:** Create a new postmaster filter




---

#### 

- **Path:** `/api/postmaster-filters/:name`
- **Method:** `GET`
- **Description:** Get postmaster filter details




---

#### 

- **Path:** `/api/postmaster-filters/:name`
- **Method:** `PUT`
- **Description:** Update an existing postmaster filter




---

#### 

- **Path:** `/api/postmaster-filters/:name`
- **Method:** `DELETE`
- **Description:** Delete a postmaster filter




---

#### 

- **Path:** `/notification-events`
- **Method:** `GET`
- **Description:** Display notification events management page




---

#### 

- **Path:** `/notification-events/new`
- **Method:** `GET`
- **Description:** Display new notification event form




---

#### 

- **Path:** `/notification-events/:id`
- **Method:** `GET`
- **Description:** Display notification event edit form




---

#### 

- **Path:** `/api/notification-events`
- **Method:** `POST`
- **Description:** Create a new notification event




---

#### 

- **Path:** `/api/notification-events/:id`
- **Method:** `GET`
- **Description:** Get notification event details




---

#### 

- **Path:** `/api/notification-events/:id`
- **Method:** `PUT`
- **Description:** Update an existing notification event




---

#### 

- **Path:** `/api/notification-events/:id`
- **Method:** `DELETE`
- **Description:** Delete a notification event




---

#### 

- **Path:** `/acl`
- **Method:** `GET`
- **Description:** Display ACL management page




---

#### 

- **Path:** `/api/acl`
- **Method:** `POST`
- **Description:** Create a new ACL




---

#### 

- **Path:** `/api/acl/:id`
- **Method:** `GET`
- **Description:** Get ACL details




---

#### 

- **Path:** `/api/acl/:id`
- **Method:** `PUT`
- **Description:** Update an existing ACL




---

#### 

- **Path:** `/api/acl/:id`
- **Method:** `DELETE`
- **Description:** Delete an ACL




---

#### 

- **Path:** `/ticket-attribute-relations`
- **Method:** `GET`
- **Description:** Display ticket attribute relations management page




---

#### 

- **Path:** `/ticket-attribute-relations/new`
- **Method:** `GET`
- **Description:** Display new ticket attribute relation form




---

#### 

- **Path:** `/ticket-attribute-relations/:id`
- **Method:** `GET`
- **Description:** Display ticket attribute relation edit form




---

#### 

- **Path:** `/ticket-attribute-relations/:id/download`
- **Method:** `GET`
- **Description:** Download ticket attribute relation file




---

#### 

- **Path:** `/api/ticket-attribute-relations`
- **Method:** `POST`
- **Description:** Create a new ticket attribute relation




---

#### 

- **Path:** `/api/ticket-attribute-relations/:id`
- **Method:** `PUT`
- **Description:** Update an existing ticket attribute relation




---

#### 

- **Path:** `/api/ticket-attribute-relations/:id`
- **Method:** `DELETE`
- **Description:** Delete a ticket attribute relation




---

#### 

- **Path:** `/api/ticket-attribute-relations/reorder`
- **Method:** `POST`
- **Description:** Reorder ticket attribute relations via drag-and-drop




---

#### 

- **Path:** `/api/ticket-attribute-relations/evaluate`
- **Method:** `GET`
- **Description:** Evaluate ticket attribute relations for filtering




---

#### 

- **Path:** `/generic-agent`
- **Method:** `GET`
- **Description:** Display generic agent jobs management page




---

#### 

- **Path:** `/api/generic-agent`
- **Method:** `POST`
- **Description:** Create a new generic agent job




---

#### 

- **Path:** `/api/generic-agent/:name`
- **Method:** `GET`
- **Description:** Get generic agent job details




---

#### 

- **Path:** `/api/generic-agent/:name`
- **Method:** `PUT`
- **Description:** Update an existing generic agent job




---

#### 

- **Path:** `/api/generic-agent/:name`
- **Method:** `DELETE`
- **Description:** Delete a generic agent job




---

#### 

- **Path:** `/customer-groups`
- **Method:** `GET`
- **Description:** Display customer groups management page




---

#### 

- **Path:** `/customer-groups/customer/:id`
- **Method:** `GET`
- **Description:** Edit group permissions for a customer company




---

#### 

- **Path:** `/customer-groups/customer/:id`
- **Method:** `POST`
- **Description:** Update group permissions for a customer company




---

#### 

- **Path:** `/customer-groups/group/:id`
- **Method:** `GET`
- **Description:** Edit customer permissions for a group




---

#### 

- **Path:** `/customer-groups/group/:id`
- **Method:** `POST`
- **Description:** Update customer permissions for a group




---

#### 

- **Path:** `/api/customer-groups/permissions`
- **Method:** `GET`
- **Description:** Get customer group permissions




---

#### 

- **Path:** `/customer-user-groups`
- **Method:** `GET`
- **Description:** Display customer user groups management page




---

#### 

- **Path:** `/customer-user-groups/user/:id`
- **Method:** `GET`
- **Description:** Edit group permissions for a customer user




---

#### 

- **Path:** `/customer-user-groups/user/:id`
- **Method:** `POST`
- **Description:** Update group permissions for a customer user




---

#### 

- **Path:** `/customer-user-groups/group/:id`
- **Method:** `GET`
- **Description:** Edit customer user permissions for a group




---

#### 

- **Path:** `/customer-user-groups/group/:id`
- **Method:** `POST`
- **Description:** Update customer user permissions for a group




---

#### 

- **Path:** `/api/customer-user-groups/permissions`
- **Method:** `GET`
- **Description:** Get customer user group permissions




---

#### 

- **Path:** `/webservices`
- **Method:** `GET`
- **Description:** Display web services management page




---

#### 

- **Path:** `/webservices/new`
- **Method:** `GET`
- **Description:** Display new web service form




---

#### 

- **Path:** `/webservices/:id`
- **Method:** `GET`
- **Description:** Display web service edit form




---

#### 

- **Path:** `/webservices/:id/history`
- **Method:** `GET`
- **Description:** Display web service configuration history




---

#### 

- **Path:** `/api/webservices`
- **Method:** `POST`
- **Description:** Create a new web service




---

#### 

- **Path:** `/api/webservices/:id`
- **Method:** `GET`
- **Description:** Get web service details




---

#### 

- **Path:** `/api/webservices/:id`
- **Method:** `PUT`
- **Description:** Update an existing web service




---

#### 

- **Path:** `/api/webservices/:id`
- **Method:** `DELETE`
- **Description:** Delete a web service




---

#### 

- **Path:** `/api/webservices/:id/test`
- **Method:** `POST`
- **Description:** Test web service connection




---

#### 

- **Path:** `/api/webservices/:id/history/:historyId/restore`
- **Method:** `POST`
- **Description:** Restore web service from history




---

#### 

- **Path:** `/api/dynamic-fields/:id/autocomplete`
- **Method:** `GET`
- **Description:** Autocomplete for webservice-backed dynamic fields




---

#### 

- **Path:** `/api/dynamic-fields/:id/webservice-test`
- **Method:** `POST`
- **Description:** Test webservice configuration for a dynamic field




---

#### 

- **Path:** `/sessions`
- **Method:** `GET`
- **Description:** Display session management page




---

#### 

- **Path:** `/api/sessions/:id`
- **Method:** `DELETE`
- **Description:** Terminate a specific session




---

#### 

- **Path:** `/api/sessions/user/:user_id`
- **Method:** `DELETE`
- **Description:** Terminate all sessions for a user




---

#### 

- **Path:** `/api/sessions`
- **Method:** `DELETE`
- **Description:** Terminate all sessions (emergency)




---

#### 

- **Path:** `/system-maintenance`
- **Method:** `GET`
- **Description:** Display system maintenance list




---

#### 

- **Path:** `/system-maintenance/new`
- **Method:** `GET`
- **Description:** Display create maintenance form




---

#### 

- **Path:** `/system-maintenance/:id/edit`
- **Method:** `GET`
- **Description:** Display edit maintenance form




---

#### 

- **Path:** `/api/system-maintenance`
- **Method:** `POST`
- **Description:** Create new maintenance record




---

#### 

- **Path:** `/api/system-maintenance/:id`
- **Method:** `GET`
- **Description:** Get maintenance record details




---

#### 

- **Path:** `/api/system-maintenance/:id`
- **Method:** `PUT`
- **Description:** Update maintenance record




---

#### 

- **Path:** `/api/system-maintenance/:id`
- **Method:** `DELETE`
- **Description:** Delete maintenance record




---

#### 

- **Path:** `/plugins`
- **Method:** `GET`
- **Description:** Display plugin management page




---

#### 

- **Path:** `/plugins/logs`
- **Method:** `GET`
- **Description:** Display plugin logs viewer




---

#### 

- **Path:** `/api/users/:id/2fa/disable`
- **Method:** `POST`
- **Description:** Admin override: disable 2FA for a user (requires reason)




---

#### 

- **Path:** `/api/customers/:login/2fa/disable`
- **Method:** `POST`
- **Description:** Admin override: disable 2FA for a customer (requires reason)




---



### Default: admin-dynamic-aliases

**Description:** Friendly URLs for dynamic admin modules  
**Prefix:** `/admin`  
**Middleware:** `auth` `admin` 


#### 

- **Path:** `/mail-accounts`
- **Method:** `GET`
- **Description:** List mail accounts




---

#### 

- **Path:** `/mail-accounts/export`
- **Method:** `GET`
- **Description:** Export mail accounts




---

#### 

- **Path:** `/mail-accounts/new`
- **Method:** `GET`
- **Description:** Render mail account creation form




---

#### 

- **Path:** `/mail-accounts/:id`
- **Method:** `GET`
- **Description:** Show mail account details




---

#### 

- **Path:** `/mail-accounts/:id/edit`
- **Method:** `GET`
- **Description:** Render mail account edit form




---

#### 

- **Path:** `/mail-accounts`
- **Method:** `POST`
- **Description:** Create a mail account




---

#### 

- **Path:** `/mail-accounts/:id`
- **Method:** `PUT`
- **Description:** Update a mail account




---

#### 

- **Path:** `/mail-accounts/:id`
- **Method:** `POST`
- **Description:** Update a mail account via form submission




---

#### 

- **Path:** `/mail-accounts/:id`
- **Method:** `DELETE`
- **Description:** Delete a mail account




---

#### 

- **Path:** `/mail-accounts/:id/status`
- **Method:** `PUT`
- **Description:** Toggle mail account status




---

#### 

- **Path:** `/mail-accounts/:id/status`
- **Method:** `POST`
- **Description:** Toggle mail account status via form submission




---

#### 

- **Path:** `/mail-accounts/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom mail account action




---

#### 

- **Path:** `/mail-accounts/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom mail account action




---

#### 

- **Path:** `/communication-channels`
- **Method:** `GET`
- **Description:** List communication channels




---

#### 

- **Path:** `/communication-channels/export`
- **Method:** `GET`
- **Description:** Export communication channels




---

#### 

- **Path:** `/communication-channels/new`
- **Method:** `GET`
- **Description:** Render communication channel creation form




---

#### 

- **Path:** `/communication-channels/:id`
- **Method:** `GET`
- **Description:** Show communication channel details




---

#### 

- **Path:** `/communication-channels/:id/edit`
- **Method:** `GET`
- **Description:** Render communication channel edit form




---

#### 

- **Path:** `/communication-channels`
- **Method:** `POST`
- **Description:** Create a communication channel




---

#### 

- **Path:** `/communication-channels/:id`
- **Method:** `PUT`
- **Description:** Update a communication channel




---

#### 

- **Path:** `/communication-channels/:id`
- **Method:** `POST`
- **Description:** Update a communication channel via form submission




---

#### 

- **Path:** `/communication-channels/:id`
- **Method:** `DELETE`
- **Description:** Delete a communication channel




---

#### 

- **Path:** `/communication-channels/:id/status`
- **Method:** `PUT`
- **Description:** Toggle communication channel status




---

#### 

- **Path:** `/communication-channels/:id/status`
- **Method:** `POST`
- **Description:** Toggle communication channel status via form submission




---

#### 

- **Path:** `/communication-channels/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom communication channel action




---

#### 

- **Path:** `/communication-channels/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom communication channel action




---

#### 

- **Path:** `/package-repositories`
- **Method:** `GET`
- **Description:** List package repositories




---

#### 

- **Path:** `/package-repositories/export`
- **Method:** `GET`
- **Description:** Export package repositories




---

#### 

- **Path:** `/package-repositories/new`
- **Method:** `GET`
- **Description:** Render package repository creation form




---

#### 

- **Path:** `/package-repositories/:id`
- **Method:** `GET`
- **Description:** Show package repository details




---

#### 

- **Path:** `/package-repositories/:id/edit`
- **Method:** `GET`
- **Description:** Render package repository edit form




---

#### 

- **Path:** `/package-repositories`
- **Method:** `POST`
- **Description:** Create a package repository




---

#### 

- **Path:** `/package-repositories/:id`
- **Method:** `PUT`
- **Description:** Update a package repository




---

#### 

- **Path:** `/package-repositories/:id`
- **Method:** `POST`
- **Description:** Update a package repository via form submission




---

#### 

- **Path:** `/package-repositories/:id`
- **Method:** `DELETE`
- **Description:** Delete a package repository




---

#### 

- **Path:** `/package-repositories/:id/status`
- **Method:** `PUT`
- **Description:** Toggle package repository status




---

#### 

- **Path:** `/package-repositories/:id/status`
- **Method:** `POST`
- **Description:** Toggle package repository status via form submission




---

#### 

- **Path:** `/package-repositories/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom package repository action




---

#### 

- **Path:** `/package-repositories/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom package repository action




---

#### 

- **Path:** `/auto-responses`
- **Method:** `GET`
- **Description:** List auto responses




---

#### 

- **Path:** `/auto-responses/export`
- **Method:** `GET`
- **Description:** Export auto responses




---

#### 

- **Path:** `/auto-responses/new`
- **Method:** `GET`
- **Description:** Render auto response creation form




---

#### 

- **Path:** `/auto-responses/:id`
- **Method:** `GET`
- **Description:** Show auto response details




---

#### 

- **Path:** `/auto-responses/:id/edit`
- **Method:** `GET`
- **Description:** Render auto response edit form




---

#### 

- **Path:** `/auto-responses`
- **Method:** `POST`
- **Description:** Create an auto response




---

#### 

- **Path:** `/auto-responses/:id`
- **Method:** `PUT`
- **Description:** Update an auto response




---

#### 

- **Path:** `/auto-responses/:id`
- **Method:** `POST`
- **Description:** Update an auto response via form submission




---

#### 

- **Path:** `/auto-responses/:id`
- **Method:** `DELETE`
- **Description:** Delete an auto response




---

#### 

- **Path:** `/auto-responses/:id/status`
- **Method:** `PUT`
- **Description:** Toggle auto response status




---

#### 

- **Path:** `/auto-responses/:id/status`
- **Method:** `POST`
- **Description:** Toggle auto response status via form submission




---

#### 

- **Path:** `/auto-responses/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom auto response action




---

#### 

- **Path:** `/auto-responses/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom auto response action




---

#### 

- **Path:** `/auto-response-types`
- **Method:** `GET`
- **Description:** List auto response types




---

#### 

- **Path:** `/auto-response-types/export`
- **Method:** `GET`
- **Description:** Export auto response types




---

#### 

- **Path:** `/auto-response-types/new`
- **Method:** `GET`
- **Description:** Render auto response type creation form




---

#### 

- **Path:** `/auto-response-types/:id`
- **Method:** `GET`
- **Description:** Show auto response type details




---

#### 

- **Path:** `/auto-response-types/:id/edit`
- **Method:** `GET`
- **Description:** Render auto response type edit form




---

#### 

- **Path:** `/auto-response-types`
- **Method:** `POST`
- **Description:** Create an auto response type




---

#### 

- **Path:** `/auto-response-types/:id`
- **Method:** `PUT`
- **Description:** Update an auto response type




---

#### 

- **Path:** `/auto-response-types/:id`
- **Method:** `POST`
- **Description:** Update an auto response type via form submission




---

#### 

- **Path:** `/auto-response-types/:id`
- **Method:** `DELETE`
- **Description:** Delete an auto response type




---

#### 

- **Path:** `/auto-response-types/:id/status`
- **Method:** `PUT`
- **Description:** Toggle auto response type status




---

#### 

- **Path:** `/auto-response-types/:id/status`
- **Method:** `POST`
- **Description:** Toggle auto response type status via form submission




---

#### 

- **Path:** `/auto-response-types/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom auto response type action




---

#### 

- **Path:** `/auto-response-types/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom auto response type action




---

#### 

- **Path:** `/follow-up-options`
- **Method:** `GET`
- **Description:** List follow up options




---

#### 

- **Path:** `/follow-up-options/export`
- **Method:** `GET`
- **Description:** Export follow up options




---

#### 

- **Path:** `/follow-up-options/new`
- **Method:** `GET`
- **Description:** Render follow up option creation form




---

#### 

- **Path:** `/follow-up-options/:id`
- **Method:** `GET`
- **Description:** Show follow up option details




---

#### 

- **Path:** `/follow-up-options/:id/edit`
- **Method:** `GET`
- **Description:** Render follow up option edit form




---

#### 

- **Path:** `/follow-up-options`
- **Method:** `POST`
- **Description:** Create a follow up option




---

#### 

- **Path:** `/follow-up-options/:id`
- **Method:** `PUT`
- **Description:** Update a follow up option




---

#### 

- **Path:** `/follow-up-options/:id`
- **Method:** `POST`
- **Description:** Update a follow up option via form submission




---

#### 

- **Path:** `/follow-up-options/:id`
- **Method:** `DELETE`
- **Description:** Delete a follow up option




---

#### 

- **Path:** `/follow-up-options/:id/status`
- **Method:** `PUT`
- **Description:** Toggle follow up option status




---

#### 

- **Path:** `/follow-up-options/:id/status`
- **Method:** `POST`
- **Description:** Toggle follow up option status via form submission




---

#### 

- **Path:** `/follow-up-options/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom follow up option action




---

#### 

- **Path:** `/follow-up-options/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom follow up option action




---

#### 

- **Path:** `/link-states`
- **Method:** `GET`
- **Description:** List link states




---

#### 

- **Path:** `/link-states/export`
- **Method:** `GET`
- **Description:** Export link states




---

#### 

- **Path:** `/link-states/new`
- **Method:** `GET`
- **Description:** Render link state creation form




---

#### 

- **Path:** `/link-states/:id`
- **Method:** `GET`
- **Description:** Show link state details




---

#### 

- **Path:** `/link-states/:id/edit`
- **Method:** `GET`
- **Description:** Render link state edit form




---

#### 

- **Path:** `/link-states`
- **Method:** `POST`
- **Description:** Create a link state




---

#### 

- **Path:** `/link-states/:id`
- **Method:** `PUT`
- **Description:** Update a link state




---

#### 

- **Path:** `/link-states/:id`
- **Method:** `POST`
- **Description:** Update a link state via form submission




---

#### 

- **Path:** `/link-states/:id`
- **Method:** `DELETE`
- **Description:** Delete a link state




---

#### 

- **Path:** `/link-states/:id/status`
- **Method:** `PUT`
- **Description:** Toggle link state status




---

#### 

- **Path:** `/link-states/:id/status`
- **Method:** `POST`
- **Description:** Toggle link state status via form submission




---

#### 

- **Path:** `/link-states/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom link state action




---

#### 

- **Path:** `/link-states/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom link state action




---

#### 

- **Path:** `/link-types`
- **Method:** `GET`
- **Description:** List link types




---

#### 

- **Path:** `/link-types/export`
- **Method:** `GET`
- **Description:** Export link types




---

#### 

- **Path:** `/link-types/new`
- **Method:** `GET`
- **Description:** Render link type creation form




---

#### 

- **Path:** `/link-types/:id`
- **Method:** `GET`
- **Description:** Show link type details




---

#### 

- **Path:** `/link-types/:id/edit`
- **Method:** `GET`
- **Description:** Render link type edit form




---

#### 

- **Path:** `/link-types`
- **Method:** `POST`
- **Description:** Create a link type




---

#### 

- **Path:** `/link-types/:id`
- **Method:** `PUT`
- **Description:** Update a link type




---

#### 

- **Path:** `/link-types/:id`
- **Method:** `POST`
- **Description:** Update a link type via form submission




---

#### 

- **Path:** `/link-types/:id`
- **Method:** `DELETE`
- **Description:** Delete a link type




---

#### 

- **Path:** `/link-types/:id/status`
- **Method:** `PUT`
- **Description:** Toggle link type status




---

#### 

- **Path:** `/link-types/:id/status`
- **Method:** `POST`
- **Description:** Toggle link type status via form submission




---

#### 

- **Path:** `/link-types/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom link type action




---

#### 

- **Path:** `/link-types/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom link type action




---

#### 

- **Path:** `/queue-auto-responses`
- **Method:** `GET`
- **Description:** List queue auto responses




---

#### 

- **Path:** `/queue-auto-responses/export`
- **Method:** `GET`
- **Description:** Export queue auto responses




---

#### 

- **Path:** `/queue-auto-responses/new`
- **Method:** `GET`
- **Description:** Render queue auto response creation form




---

#### 

- **Path:** `/queue-auto-responses/:id`
- **Method:** `GET`
- **Description:** Show queue auto response details




---

#### 

- **Path:** `/queue-auto-responses/:id/edit`
- **Method:** `GET`
- **Description:** Render queue auto response edit form




---

#### 

- **Path:** `/queue-auto-responses`
- **Method:** `POST`
- **Description:** Create a queue auto response




---

#### 

- **Path:** `/queue-auto-responses/:id`
- **Method:** `PUT`
- **Description:** Update a queue auto response




---

#### 

- **Path:** `/queue-auto-responses/:id`
- **Method:** `POST`
- **Description:** Update a queue auto response via form submission




---

#### 

- **Path:** `/queue-auto-responses/:id`
- **Method:** `DELETE`
- **Description:** Delete a queue auto response




---

#### 

- **Path:** `/queue-auto-responses/:id/status`
- **Method:** `PUT`
- **Description:** Toggle queue auto response status




---

#### 

- **Path:** `/queue-auto-responses/:id/status`
- **Method:** `POST`
- **Description:** Toggle queue auto response status via form submission




---

#### 

- **Path:** `/queue-auto-responses/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom queue auto response action




---

#### 

- **Path:** `/queue-auto-responses/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom queue auto response action




---

#### 

- **Path:** `/article-colors`
- **Method:** `GET`
- **Description:** List article colors




---

#### 

- **Path:** `/article-colors/export`
- **Method:** `GET`
- **Description:** Export article colors




---

#### 

- **Path:** `/article-colors/new`
- **Method:** `GET`
- **Description:** Render article color creation form




---

#### 

- **Path:** `/article-colors/:id`
- **Method:** `GET`
- **Description:** Show article color details




---

#### 

- **Path:** `/article-colors/:id/edit`
- **Method:** `GET`
- **Description:** Render article color edit form




---

#### 

- **Path:** `/article-colors`
- **Method:** `POST`
- **Description:** Create an article color




---

#### 

- **Path:** `/article-colors/:id`
- **Method:** `PUT`
- **Description:** Update an article color




---

#### 

- **Path:** `/article-colors/:id`
- **Method:** `POST`
- **Description:** Update an article color via form submission




---

#### 

- **Path:** `/article-colors/:id`
- **Method:** `DELETE`
- **Description:** Delete an article color




---

#### 

- **Path:** `/article-colors/:id/:action`
- **Method:** `GET`
- **Description:** Execute custom article color action




---

#### 

- **Path:** `/article-colors/:id/:action`
- **Method:** `POST`
- **Description:** Execute custom article color action




---



### Default: admin-mail-account-status

**Description:** Mail account poll status endpoints  
**Prefix:** `/admin`  
**Middleware:** `auth` `admin` 


#### 

- **Path:** `/mail-accounts/:id/poll-status`
- **Method:** `GET`
- **Description:** Fetch poll status for a mail account




---

#### 

- **Path:** `/dynamic/mail_account/:id/poll-status`
- **Method:** `GET`
- **Description:** Fetch poll status for a mail account (dynamic path)




---



### Default: agent

**Description:** Agent-specific routes for ticket management  
**Prefix:** `/agent`  
**Middleware:** `auth` `queue_ro` 


#### 

- **Path:** `/tickets`
- **Method:** `GET`
- **Description:** Display agent tickets list with full functionality




---

#### 

- **Path:** `/tickets/:id/reply`
- **Method:** `POST`
- **Description:** Process customer reply to ticket




---

#### 

- **Path:** `/tickets/:id/note`
- **Method:** `POST`
- **Description:** Add internal note to ticket




---

#### 

- **Path:** `/tickets/:id/phone`
- **Method:** `POST`
- **Description:** Record phone call for ticket




---

#### 

- **Path:** `/tickets/:id/status`
- **Method:** `POST`
- **Description:** Update ticket status




---

#### 

- **Path:** `/tickets/:id/assign`
- **Method:** `POST`
- **Description:** Assign ticket to specific agent




---

#### 

- **Path:** `/tickets/:id/priority`
- **Method:** `POST`
- **Description:** Update ticket priority




---

#### 

- **Path:** `/tickets/:id/queue`
- **Method:** `POST`
- **Description:** Move ticket to different queue




---

#### 

- **Path:** `/tickets/:id/merge`
- **Method:** `POST`
- **Description:** Merge ticket with another ticket




---

#### 

- **Path:** `/tickets/:id/split`
- **Method:** `POST`
- **Description:** Split ticket into multiple tickets (temporary)




---

#### 

- **Path:** `/tickets/:id/customer-users`
- **Method:** `GET`
- **Description:** Get customer users associated with ticket




---

#### 

- **Path:** `/tickets/:id/history`
- **Method:** `GET`
- **Description:** HTMX fragment: ticket history




---

#### 

- **Path:** `/tickets/:id/links`
- **Method:** `GET`
- **Description:** HTMX fragment: ticket links




---

#### 

- **Path:** `/tickets/:id/draft`
- **Method:** `POST`
- **Description:** Save draft reply for ticket




---

#### 

- **Path:** `/tickets/bulk/status`
- **Method:** `POST`
- **Description:** Bulk update ticket status




---

#### 

- **Path:** `/tickets/bulk/priority`
- **Method:** `POST`
- **Description:** Bulk update ticket priority




---

#### 

- **Path:** `/tickets/bulk/queue`
- **Method:** `POST`
- **Description:** Bulk move tickets to queue




---

#### 

- **Path:** `/tickets/bulk/assign`
- **Method:** `POST`
- **Description:** Bulk assign tickets to agent




---

#### 

- **Path:** `/tickets/bulk/lock`
- **Method:** `POST`
- **Description:** Bulk lock/unlock tickets




---

#### 

- **Path:** `/tickets/bulk/merge`
- **Method:** `POST`
- **Description:** Bulk merge tickets into target




---

#### 

- **Path:** `/api/bulk-options`
- **Method:** `GET`
- **Description:** Get options for bulk action modals




---

#### 

- **Path:** `/api/tickets/ids`
- **Method:** `GET`
- **Description:** Get all ticket IDs matching current filter for bulk selection




---

#### 

- **Path:** `/queues`
- **Method:** `GET`
- **Description:** Display agent queues




---

#### 

- **Path:** `/api/templates`
- **Method:** `GET`
- **Description:** Get templates for a queue (query params: queue_id, type)




---

#### 

- **Path:** `/api/templates/:id`
- **Method:** `GET`
- **Description:** Get a single template with variable substitution




---

#### 

- **Path:** `/api/signatures/queue/:queue_id`
- **Method:** `GET`
- **Description:** Get a queue signature with variable substitution




---

#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `GET`
- **Description:** Get current session timeout preference




---

#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `POST`
- **Description:** Update session timeout preference




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `GET`
- **Description:** Get current language preference and available languages




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `POST`
- **Description:** Update language preference




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `GET`
- **Description:** Get current theme preference and available themes




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `POST`
- **Description:** Update theme preference (persists to database)




---

#### 

- **Path:** `/api/preferences/reminders-enabled`
- **Method:** `GET`
- **Description:** Get ticket reminders enabled preference




---

#### 

- **Path:** `/api/preferences/reminders-enabled`
- **Method:** `POST`
- **Description:** Update ticket reminders enabled preference




---

#### 

- **Path:** `/api/profile`
- **Method:** `GET`
- **Description:** Get current user&#39;s profile information




---

#### 

- **Path:** `/api/profile`
- **Method:** `POST`
- **Description:** Update current user&#39;s profile information




---

#### 

- **Path:** `/password`
- **Method:** `GET`
- **Description:** Display password change form




---

#### 

- **Path:** `/password/change`
- **Method:** `POST`
- **Description:** Process password change




---



### Default: api-aliases-protected

**Description:** Legacy /api alias routes - most moved to dedicated files  
**Prefix:** `/api`  
**Middleware:** `unified_auth` 




### Default: api-attachments

**Description:** Ticket attachment API endpoints  
**Prefix:** `/api`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/tickets/:id/attachments`
- **Method:** `GET`
- **Description:** List attachments for a ticket




---

#### 

- **Path:** `/tickets/:id/attachments`
- **Method:** `POST`
- **Description:** Upload attachment to a ticket




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id`
- **Method:** `GET`
- **Description:** Download a specific attachment




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id`
- **Method:** `DELETE`
- **Description:** Delete a specific attachment




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id/thumbnail`
- **Method:** `GET`
- **Description:** Get thumbnail/preview for an image attachment




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id/view`
- **Method:** `GET`
- **Description:** Inline viewer for common attachment types




---



### Default: api-canned-responses-protected

**Description:** Canned response management API endpoints (protected)  
**Prefix:** `/api/canned-responses`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** List all canned responses




---

#### 

- **Path:** `/quick`
- **Method:** `GET`
- **Description:** Get quick access responses




---

#### 

- **Path:** `/popular`
- **Method:** `GET`
- **Description:** Get popular responses




---

#### 

- **Path:** `/categories`
- **Method:** `GET`
- **Description:** List response categories




---

#### 

- **Path:** `/category/:category`
- **Method:** `GET`
- **Description:** Get responses by category




---

#### 

- **Path:** `/search`
- **Method:** `GET`
- **Description:** Search canned responses




---

#### 

- **Path:** `/user`
- **Method:** `GET`
- **Description:** Get responses for current user




---

#### 

- **Path:** `/:id`
- **Method:** `GET`
- **Description:** Get response by ID




---



### Default: api-customers-protected

**Description:** Customer management API endpoints (protected)  
**Prefix:** `/api/customers`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/search`
- **Method:** `GET`
- **Description:** Search customers (autocomplete)




---



### Default: api-dashboard-protected

**Description:** Dashboard widget endpoints (protected)  
**Prefix:** `/api/dashboard`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/stats`
- **Method:** `GET`
- **Description:** Dashboard statistics widget




---

#### 

- **Path:** `/recent-tickets`
- **Method:** `GET`
- **Description:** Recent tickets widget




---

#### 

- **Path:** `/notifications`
- **Method:** `GET`
- **Description:** Notifications widget




---

#### 

- **Path:** `/quick-actions`
- **Method:** `GET`
- **Description:** Quick actions widget




---

#### 

- **Path:** `/activity`
- **Method:** `GET`
- **Description:** Activity feed widget




---

#### 

- **Path:** `/activity-stream`
- **Method:** `GET`
- **Description:** Activity stream endpoint




---

#### 

- **Path:** `/performance`
- **Method:** `GET`
- **Description:** Performance metrics widget




---

#### 

- **Path:** `/widgets`
- **Method:** `GET`
- **Description:** List all available dashboard widgets with config




---

#### 

- **Path:** `/widgets/config`
- **Method:** `GET`
- **Description:** Get user&#39;s dashboard widget configuration




---

#### 

- **Path:** `/widgets/config`
- **Method:** `POST`
- **Description:** Update user&#39;s dashboard widget configuration




---



### Default: api-files-protected

**Description:** File serving endpoint (protected)  
**Prefix:** `/api/files`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/*path`
- **Method:** `GET`
- **Description:** Serve uploaded files




---



### Default: api-groups-protected

**Description:** Group management API endpoints (protected)  
**Prefix:** `/api/groups`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** List all groups




---

#### 

- **Path:** `/:id`
- **Method:** `GET`
- **Description:** Get group by ID




---

#### 

- **Path:** `/:id/members`
- **Method:** `GET`
- **Description:** Get group members




---



### Default: api-lookups

**Description:** Lookup API endpoints  
**Prefix:** `/api`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/lookups/queues`
- **Method:** `GET`
- **Description:** Get list of queues




---

#### 

- **Path:** `/lookups/priorities`
- **Method:** `GET`
- **Description:** Get list of priorities




---

#### 

- **Path:** `/lookups/types`
- **Method:** `GET`
- **Description:** Get list of ticket types




---

#### 

- **Path:** `/lookups/statuses`
- **Method:** `GET`
- **Description:** Get list of ticket statuses




---

#### 

- **Path:** `/lookups/form-data`
- **Method:** `GET`
- **Description:** Get form data for ticket creation




---

#### 

- **Path:** `/lookups/cache/invalidate`
- **Method:** `POST`
- **Description:** Invalidate lookup cache




---



### Default: api-mcp

**Description:** Model Context Protocol endpoint for AI assistant integration  
**Prefix:** `/api`  
**Middleware:** 


#### 

- **Path:** `/mcp`
- **Method:** `POST`
- **Description:** MCP JSON-RPC endpoint




---

#### 

- **Path:** `/mcp`
- **Method:** `GET`
- **Description:** MCP endpoint info




---



### Default: api-notifications

**Description:** Notification API endpoints  
**Prefix:** `/api`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/notifications/pending`
- **Method:** `GET`
- **Description:** Fetch pending reminder notifications for current agent




---



### Default: api-queues

**Description:** Queue API endpoints for frontend  
**Prefix:** `/api/queues`  
**Middleware:** `unified_auth` `queue_ro` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** List all queues




---

#### 

- **Path:** `/`
- **Method:** `POST`
- **Description:** Create new queue




---

#### 

- **Path:** `/:id`
- **Method:** `GET`
- **Description:** Get queue by ID




---

#### 

- **Path:** `/:id/details`
- **Method:** `GET`
- **Description:** Get queue details




---

#### 

- **Path:** `/:id/status`
- **Method:** `PUT`
- **Description:** Update queue status




---



### Default: api-ticket-messages

**Description:** Ticket message retrieval and creation endpoints  
**Prefix:** `/api`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/tickets/:id/messages`
- **Method:** `GET`
- **Description:** List ticket messages including attachments




---

#### 

- **Path:** `/tickets/:id/messages`
- **Method:** `POST`
- **Description:** Create a new message for a ticket




---



### Default: api-tickets-protected

**Description:** Ticket management API endpoints (protected)  
**Prefix:** `/api/tickets`  
**Middleware:** `unified_auth` `scope_tickets_read` `queue_ro` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** List tickets with filtering




---

#### 

- **Path:** `/`
- **Method:** `POST`
- **Description:** Create new ticket




---

#### 

- **Path:** `/:id`
- **Method:** `GET`
- **Description:** Get ticket by ID




---

#### 

- **Path:** `/:id`
- **Method:** `PUT`
- **Description:** Update ticket




---

#### 

- **Path:** `/:id`
- **Method:** `DELETE`
- **Description:** Delete ticket




---

#### 

- **Path:** `/:id/notes`
- **Method:** `POST`
- **Description:** Add note to ticket




---

#### 

- **Path:** `/:id/history`
- **Method:** `GET`
- **Description:** Get ticket history




---

#### 

- **Path:** `/:id/available-agents`
- **Method:** `GET`
- **Description:** Get available agents for assignment




---

#### 

- **Path:** `/:id/assign`
- **Method:** `POST`
- **Description:** Assign ticket to agent




---

#### 

- **Path:** `/:id/close`
- **Method:** `POST`
- **Description:** Close ticket




---

#### 

- **Path:** `/:id/reopen`
- **Method:** `POST`
- **Description:** Reopen closed ticket




---

#### 

- **Path:** `/:id/status`
- **Method:** `POST`
- **Description:** Update ticket status (supports pending reminders)




---

#### 

- **Path:** `/:id/time`
- **Method:** `POST`
- **Description:** Add time accounting entry




---

#### 

- **Path:** `/:id/reply`
- **Method:** `POST`
- **Description:** Reply to ticket




---

#### 

- **Path:** `/:id/priority`
- **Method:** `POST`
- **Description:** Update ticket priority




---

#### 

- **Path:** `/:id/queue`
- **Method:** `POST`
- **Description:** Update ticket queue




---

#### 

- **Path:** `/search`
- **Method:** `GET`
- **Description:** Search tickets




---

#### 

- **Path:** `/filter`
- **Method:** `GET`
- **Description:** Filter tickets




---

#### 

- **Path:** `/advanced-search`
- **Method:** `GET`
- **Description:** Advanced ticket search




---

#### 

- **Path:** `/search/suggestions`
- **Method:** `GET`
- **Description:** Get search suggestions




---

#### 

- **Path:** `/search/export`
- **Method:** `GET`
- **Description:** Export search results




---

#### 

- **Path:** `/search/history`
- **Method:** `POST`
- **Description:** Save search to history




---

#### 

- **Path:** `/search/history`
- **Method:** `GET`
- **Description:** Get search history




---

#### 

- **Path:** `/search/history/:id`
- **Method:** `DELETE`
- **Description:** Delete search history item




---

#### 

- **Path:** `/search/saved`
- **Method:** `POST`
- **Description:** Create saved search




---

#### 

- **Path:** `/search/saved`
- **Method:** `GET`
- **Description:** Get saved searches




---

#### 

- **Path:** `/search/saved/:id/execute`
- **Method:** `GET`
- **Description:** Execute saved search




---

#### 

- **Path:** `/search/saved/:id`
- **Method:** `PUT`
- **Description:** Update saved search




---

#### 

- **Path:** `/search/saved/:id`
- **Method:** `DELETE`
- **Description:** Delete saved search




---

#### 

- **Path:** `/:id/merge`
- **Method:** `POST`
- **Description:** Merge tickets




---

#### 

- **Path:** `/:id/unmerge`
- **Method:** `POST`
- **Description:** Unmerge ticket




---

#### 

- **Path:** `/:id/merge-history`
- **Method:** `GET`
- **Description:** Get merge history




---



### Default: api-tokens-agent

**Description:** API token management for agents  
**Prefix:** `/api/v1/tokens`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** List my API tokens




---

#### 

- **Path:** `/`
- **Method:** `POST`
- **Description:** Create a new API token




---

#### 

- **Path:** `/:id`
- **Method:** `DELETE`
- **Description:** Revoke an API token




---

#### 

- **Path:** `/scopes`
- **Method:** `GET`
- **Description:** List available token scopes




---



### Default: api-types-protected

**Description:** Ticket type management API endpoints (protected)  
**Prefix:** `/api/types`  
**Middleware:** `unified_auth` 


#### 

- **Path:** `/`
- **Method:** `POST`
- **Description:** Create new ticket type




---

#### 

- **Path:** `/:id`
- **Method:** `PUT`
- **Description:** Update ticket type




---

#### 

- **Path:** `/:id`
- **Method:** `DELETE`
- **Description:** Delete ticket type




---



### Default: api-v1-public

**Description:** REST API v1 public endpoints  
**Prefix:** `/api/v1`  
**Middleware:** 


#### 

- **Path:** `/auth/login`
- **Method:** `POST`
- **Description:** Authenticate user and return JWT tokens




---



### Default: auth

**Description:** Authentication routes for login/logout  
**Prefix:** ``  
**Middleware:** 


#### 

- **Path:** `/login`
- **Method:** `GET`
- **Description:** Display login page with HTMX form




---

#### 

- **Path:** `/api/auth/login`
- **Method:** `POST`
- **Description:** Process login form submission via HTMX




---

#### 

- **Path:** `/api/auth/logout`
- **Method:** `POST`
- **Description:** Process logout request




---

#### 

- **Path:** `/logout`
- **Method:** `GET`
- **Description:** GET logout route that redirects to login




---

#### 

- **Path:** `/api/auth/refresh`
- **Method:** `POST`
- **Description:** Placeholder for token refresh




---

#### 

- **Path:** `/api/auth/register`
- **Method:** `POST`
- **Description:** Placeholder for user registration




---

#### 

- **Path:** `/auth/customer`
- **Method:** `GET`
- **Description:** Customer login portal




---

#### 

- **Path:** `/customer/login`
- **Method:** `GET`
- **Description:** Alias customer login path




---

#### 

- **Path:** `/customer/login`
- **Method:** `POST`
- **Description:** Process customer login form (alias)




---

#### 

- **Path:** `/api/auth/customer/login`
- **Method:** `POST`
- **Description:** Process customer login form




---

#### 

- **Path:** `/customer/logout`
- **Method:** `GET`
- **Description:** Customer logout - clears cookies and redirects




---

#### 

- **Path:** `/login/2fa`
- **Method:** `GET`
- **Description:** Display 2FA verification page during login




---

#### 

- **Path:** `/api/auth/2fa/verify`
- **Method:** `POST`
- **Description:** Verify 2FA code and complete login




---

#### 

- **Path:** `/customer/login/2fa`
- **Method:** `GET`
- **Description:** Display customer 2FA verification page during login




---

#### 

- **Path:** `/api/auth/customer/2fa/verify`
- **Method:** `POST`
- **Description:** Verify customer 2FA code and complete login




---



### Default: basic

**Description:** Basic system routes (root, health, metrics, etc.)  
**Prefix:** ``  
**Middleware:** 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Root path redirects to login




---

#### 

- **Path:** `/health`
- **Method:** `GET`
- **Description:** Basic health check endpoint




---

#### 

- **Path:** `/health/detailed`
- **Method:** `GET`
- **Description:** Detailed health check with component status




---

#### 

- **Path:** `/metrics`
- **Method:** `GET`
- **Description:** Prometheus metrics endpoint




---

#### 

- **Path:** `/queues`
- **Method:** `GET`
- **Description:** Role-aware redirect/render for queues




---

#### 

- **Path:** `/queues/:id`
- **Method:** `GET`
- **Description:** Display tickets filtered by specific queue




---

#### 

- **Path:** `/queues/:id/meta`
- **Method:** `GET`
- **Description:** Render queue metadata panel or JSON payload




---

#### 

- **Path:** `/ticket/:id`
- **Method:** `GET`
- **Description:** Display ticket detail using the unified handler




---

#### 

- **Path:** `/ticket/new`
- **Method:** `GET`
- **Description:** Redirect to email ticket creation form




---

#### 

- **Path:** `/ticket/new/email`
- **Method:** `GET`
- **Description:** Display email ticket creation form




---

#### 

- **Path:** `/ticket/new/phone`
- **Method:** `GET`
- **Description:** Display phone ticket creation form




---

#### 

- **Path:** `/healthz`
- **Method:** `GET`
- **Description:** Quick liveness probe




---

#### 

- **Path:** `/api/languages`
- **Method:** `GET`
- **Description:** Get available languages and current preference (no auth required)




---

#### 

- **Path:** `/api/languages`
- **Method:** `POST`
- **Description:** Set language preference cookie (no auth required)




---

#### 

- **Path:** `/api/themes`
- **Method:** `GET`
- **Description:** Get available themes and current preference (no auth required)




---

#### 

- **Path:** `/api/themes`
- **Method:** `POST`
- **Description:** Set theme and mode preference cookies (no auth required)




---



### Default: compatibility

**Description:** Compatibility routes for legacy URLs  
**Prefix:** ``  
**Middleware:** 


#### 

- **Path:** `/agent/tickets/:id`
- **Method:** `GET`
- **Description:** Redirect legacy agent ticket URL to unified /ticket/:tn




---

#### 

- **Path:** `/tickets/:id`
- **Method:** `GET`
- **Description:** Redirect legacy tickets URL to unified /ticket/:tn




---



### Default: customer

**Description:** Customer routes for ticket management and self-service  
**Prefix:** `/customer`  
**Middleware:** `customer-portal` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Display customer dashboard with their tickets




---

#### 

- **Path:** `/tickets`
- **Method:** `GET`
- **Description:** Display customer&#39;s ticket list




---

#### 

- **Path:** `/tickets/new`
- **Method:** `GET`
- **Description:** Display new ticket creation form




---

#### 

- **Path:** `/tickets/create`
- **Method:** `POST`
- **Description:** Process new ticket creation




---

#### 

- **Path:** `/tickets/:id`
- **Method:** `GET`
- **Description:** Display customer ticket details and conversation




---

#### 

- **Path:** `/tickets/:id/reply`
- **Method:** `POST`
- **Description:** Process customer reply to ticket




---

#### 

- **Path:** `/tickets/:id/close`
- **Method:** `POST`
- **Description:** Close customer ticket




---

#### 

- **Path:** `/profile`
- **Method:** `GET`
- **Description:** Display customer profile information




---

#### 

- **Path:** `/profile/update`
- **Method:** `POST`
- **Description:** Update customer profile information




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `GET`
- **Description:** Get customer language preference




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `POST`
- **Description:** Set customer language preference




---

#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `GET`
- **Description:** Get customer session timeout preference




---

#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `POST`
- **Description:** Set customer session timeout preference




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `GET`
- **Description:** Get customer theme preference




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `POST`
- **Description:** Set customer theme preference




---

#### 

- **Path:** `/api/preferences/wallpaper`
- **Method:** `POST`
- **Description:** Set wallpaper on/off preference for customer




---

#### 

- **Path:** `/api/preferences/coachmarks/dismiss`
- **Method:** `POST`
- **Description:** Dismiss a coachmark tip for current customer




---

#### 

- **Path:** `/api/preferences/2fa/status`
- **Method:** `GET`
- **Description:** Get 2FA status for current customer




---

#### 

- **Path:** `/api/preferences/2fa/setup`
- **Method:** `POST`
- **Description:** Initiate 2FA setup for customer - returns secret and QR code




---

#### 

- **Path:** `/api/preferences/2fa/confirm`
- **Method:** `POST`
- **Description:** Confirm 2FA setup with verification code




---

#### 

- **Path:** `/api/preferences/2fa/disable`
- **Method:** `POST`
- **Description:** Disable 2FA for customer (requires valid code)




---

#### 

- **Path:** `/password/form`
- **Method:** `GET`
- **Description:** Display password change form




---

#### 

- **Path:** `/password/change`
- **Method:** `POST`
- **Description:** Process password change




---

#### 

- **Path:** `/knowledge-base`
- **Method:** `GET`
- **Description:** Display knowledge base articles




---

#### 

- **Path:** `/kb/search`
- **Method:** `GET`
- **Description:** Search knowledge base articles




---

#### 

- **Path:** `/kb/article/:id`
- **Method:** `GET`
- **Description:** Display knowledge base article




---

#### 

- **Path:** `/tickets/:id/attachments`
- **Method:** `GET`
- **Description:** Get attachments for a customer ticket




---

#### 

- **Path:** `/tickets/:id/attachments`
- **Method:** `POST`
- **Description:** Upload attachment to a customer ticket




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id`
- **Method:** `GET`
- **Description:** Download attachment from a customer ticket




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id/thumbnail`
- **Method:** `GET`
- **Description:** Get thumbnail for an image attachment




---

#### 

- **Path:** `/tickets/:id/attachments/:attachment_id/view`
- **Method:** `GET`
- **Description:** View attachment in modal viewer




---

#### 

- **Path:** `/company/info`
- **Method:** `GET`
- **Description:** Display customer company information




---

#### 

- **Path:** `/company/users`
- **Method:** `GET`
- **Description:** Display customer company users




---



### Default: dashboard

**Description:** Dashboard routes for main application  
**Prefix:** `/dashboard`  
**Middleware:** `auth` `queue_ro` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Display main dashboard with tickets overview and activity




---

#### 

- **Path:** `/api/activity-stream`
- **Method:** `GET`
- **Description:** Server-sent events for dashboard activity updates




---

#### 

- **Path:** `/api/stats`
- **Method:** `GET`
- **Description:** Get dashboard statistics and metrics




---

#### 

- **Path:** `/api/recent-tickets`
- **Method:** `GET`
- **Description:** Get recent ticket activity for dashboard widget




---



### Default: profile

**Description:** User profile routes  
**Prefix:** `/profile`  
**Middleware:** `auth` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Display user profile page




---



### Default: redirects

**Description:** Simple redirect/alias routes  
**Prefix:** ``  
**Middleware:** `auth` 




### Default: settings

**Description:** User preferences API routes (used by profile page)  
**Prefix:** ``  
**Middleware:** `auth` 


#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `GET`
- **Description:** Get current session timeout preference




---

#### 

- **Path:** `/api/preferences/session-timeout`
- **Method:** `POST`
- **Description:** Update session timeout preference




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `GET`
- **Description:** Get current language preference and available languages




---

#### 

- **Path:** `/api/preferences/language`
- **Method:** `POST`
- **Description:** Update language preference




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `GET`
- **Description:** Get current theme preference and available themes




---

#### 

- **Path:** `/api/preferences/theme`
- **Method:** `POST`
- **Description:** Update theme preference (persists to database)




---

#### 

- **Path:** `/settings/tokens`
- **Method:** `GET`
- **Description:** Manage personal API tokens




---

#### 

- **Path:** `/api/preferences/wallpaper`
- **Method:** `POST`
- **Description:** Set wallpaper on/off preference




---

#### 

- **Path:** `/api/preferences/coachmarks/dismiss`
- **Method:** `POST`
- **Description:** Dismiss a coachmark tip for current user




---

#### 

- **Path:** `/api/preferences/2fa/status`
- **Method:** `GET`
- **Description:** Get 2FA status for current user




---

#### 

- **Path:** `/api/preferences/2fa/setup`
- **Method:** `POST`
- **Description:** Initiate 2FA setup - returns secret and QR code




---

#### 

- **Path:** `/api/preferences/2fa/confirm`
- **Method:** `POST`
- **Description:** Confirm 2FA setup with verification code




---

#### 

- **Path:** `/api/preferences/2fa/disable`
- **Method:** `POST`
- **Description:** Disable 2FA (requires valid code)




---



### Default: static

**Description:** Static routes for CSS, JS, images, and web assets  
**Prefix:** ``  
**Middleware:** 


#### 

- **Path:** `/favicon.ico`
- **Method:** `GET`
- **Description:** Serve favicon.ico file




---

#### 

- **Path:** `/favicon.svg`
- **Method:** `GET`
- **Description:** Serve favicon.svg file




---

#### 

- **Path:** `/static/*filepath`
- **Method:** `GET`
- **Description:** Serve all static files including css, js, images, webfonts




---



### Default: swagger-ui

**Description:** Swagger UI API documentation  
**Prefix:** `/swagger`  
**Middleware:** 


#### 

- **Path:** `/*any`
- **Method:** `GET`
- **Description:** Serve Swagger UI for API documentation (dark mode)




---



### Default: tickets

**Description:** Ticket management routes  
**Prefix:** `/tickets`  
**Middleware:** `auth` `queue_ro` 


#### 

- **Path:** `/`
- **Method:** `GET`
- **Description:** Display tickets list with search, filtering, and bulk actions




---

#### 

- **Path:** `/new`
- **Method:** `GET`
- **Description:** Display new ticket creation form




---

#### 

- **Path:** `/`
- **Method:** `POST`
- **Description:** Process new ticket creation




---

#### 

- **Path:** `/:id/status`
- **Method:** `PUT`
- **Description:** Update ticket status via HTMX




---

#### 

- **Path:** `/:id/comments`
- **Method:** `POST`
- **Description:** Add new comment/reply to ticket




---

#### 

- **Path:** `/:id/attachments`
- **Method:** `POST`
- **Description:** Upload file attachment to ticket




---

#### 

- **Path:** `/api/search`
- **Method:** `GET`
- **Description:** Search tickets with filters




---

#### 

- **Path:** `/:id/priority`
- **Method:** `PUT`
- **Description:** Update ticket priority




---

#### 

- **Path:** `/:id/assign`
- **Method:** `PUT`
- **Description:** Assign ticket to agent




---

#### 

- **Path:** `/customer-info/:login`
- **Method:** `GET`
- **Description:** Return customer info panel partial for selected login




---




---
*Generated by GoatFlow Route Documentation Generator*
