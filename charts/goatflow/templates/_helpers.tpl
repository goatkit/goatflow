{{/*
Expand the name of the chart.
*/}}
{{- define "goatflow.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "goatflow.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "goatflow.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "goatflow.labels" -}}
helm.sh/chart: {{ include "goatflow.chart" . }}
{{ include "goatflow.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "goatflow.selectorLabels" -}}
app.kubernetes.io/name: {{ include "goatflow.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend labels
*/}}
{{- define "goatflow.backend.labels" -}}
{{ include "goatflow.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend selector labels
*/}}
{{- define "goatflow.backend.selectorLabels" -}}
{{ include "goatflow.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Runner labels
*/}}
{{- define "goatflow.runner.labels" -}}
{{ include "goatflow.labels" . }}
app.kubernetes.io/component: runner
{{- end }}

{{/*
Runner selector labels
*/}}
{{- define "goatflow.runner.selectorLabels" -}}
{{ include "goatflow.selectorLabels" . }}
app.kubernetes.io/component: runner
{{- end }}

{{/*
Database labels
*/}}
{{- define "goatflow.database.labels" -}}
{{ include "goatflow.labels" . }}
app.kubernetes.io/component: database
{{- end }}

{{/*
Database selector labels
*/}}
{{- define "goatflow.database.selectorLabels" -}}
{{ include "goatflow.selectorLabels" . }}
app.kubernetes.io/component: database
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "goatflow.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "goatflow.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Database host
*/}}
{{- define "goatflow.database.host" -}}
{{- if .Values.database.external.enabled }}
{{- required "database.external.host is required when database.external.enabled is true" .Values.database.external.host }}
{{- else }}
{{- printf "%s-database" (include "goatflow.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "goatflow.database.port" -}}
{{- if .Values.database.external.enabled }}
{{- .Values.database.external.port | default (eq .Values.database.type "mysql" | ternary "3306" "5432") }}
{{- else if eq .Values.database.type "mysql" }}
{{- "3306" }}
{{- else }}
{{- "5432" }}
{{- end }}
{{- end }}

{{/*
Database name
*/}}
{{- define "goatflow.database.name" -}}
{{- if .Values.database.external.enabled }}
{{- .Values.database.external.database }}
{{- else if eq .Values.database.type "mysql" }}
{{- .Values.database.mysql.database }}
{{- else }}
{{- .Values.database.postgresql.database }}
{{- end }}
{{- end }}

{{/*
Database secret name
*/}}
{{- define "goatflow.database.secretName" -}}
{{- if .Values.database.external.enabled }}
{{- required "database.external.existingSecret is required when database.external.enabled is true" .Values.database.external.existingSecret }}
{{- else if eq .Values.database.type "mysql" }}
{{- .Values.database.mysql.existingSecret | default (printf "%s-database" (include "goatflow.fullname" .)) }}
{{- else }}
{{- .Values.database.postgresql.existingSecret | default (printf "%s-database" (include "goatflow.fullname" .)) }}
{{- end }}
{{- end }}

{{/*
DB_DRIVER value GoatFlow reads (mysql or postgres)
*/}}
{{- define "goatflow.database.driver" -}}
{{- if eq .Values.database.type "mysql" }}
{{- "mysql" }}
{{- else if eq .Values.database.type "postgresql" }}
{{- "postgres" }}
{{- else }}
{{- fail (printf "database.type must be mysql or postgresql, got %q" .Values.database.type) }}
{{- end }}
{{- end }}

{{/*
Prefix of the driver-scoped connection variables (internal/platform/dbconfig)
*/}}
{{- define "goatflow.database.envPrefix" -}}
{{- eq .Values.database.type "mysql" | ternary "DB_MYSQL_" "DB_PGSQL_" }}
{{- end }}

{{/*
Full name of the bundled Valkey subchart (mirrors valkey.fullname)
*/}}
{{- define "goatflow.valkey.fullname" -}}
{{- if .Values.valkey.fullnameOverride }}
{{- .Values.valkey.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default "valkey" .Values.valkey.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Valkey/Redis host
*/}}
{{- define "goatflow.valkey.host" -}}
{{- if .Values.externalValkey.enabled }}
{{- required "externalValkey.host is required when externalValkey.enabled is true" .Values.externalValkey.host }}
{{- else if .Values.valkey.enabled }}
{{- include "goatflow.valkey.fullname" . }}
{{- end }}
{{- end }}

{{/*
Valkey/Redis port
*/}}
{{- define "goatflow.valkey.port" -}}
{{- if .Values.externalValkey.enabled }}
{{- .Values.externalValkey.port | default "6379" }}
{{- else }}
{{- "6379" }}
{{- end }}
{{- end }}

{{/*
Valkey password source as "<secret name>/<key>"; empty when no password is used.
External: externalValkey.existingSecret, or the chart Secret holding externalValkey.password.
Bundled with auth: the subchart's user Secret for the "default" ACL user.
*/}}
{{- define "goatflow.valkey.passwordRef" -}}
{{- if .Values.externalValkey.enabled }}
{{- if .Values.externalValkey.existingSecret }}
{{- printf "%s/%s" .Values.externalValkey.existingSecret (.Values.externalValkey.existingSecretPasswordKey | default "valkey-password") }}
{{- else if .Values.externalValkey.password }}
{{- printf "%s-valkey-external/valkey-password" (include "goatflow.fullname" .) }}
{{- end }}
{{- else if and .Values.valkey.enabled .Values.valkey.auth.enabled }}
{{- $default := index (.Values.valkey.auth.aclUsers | default dict) "default" }}
{{- if not $default }}
{{- fail "valkey.auth.enabled needs valkey.auth.aclUsers.default: GoatFlow logs in as the default user (aclConfig alone is not supported)" }}
{{- end }}
{{- if .Values.valkey.auth.usersExistingSecret }}
{{- printf "%s/%s" .Values.valkey.auth.usersExistingSecret ($default.passwordKey | default "default") }}
{{- else }}
{{- printf "%s-auth/default-password" (include "goatflow.valkey.fullname" .) }}
{{- end }}
{{- end }}
{{- end }}

{{/*
App secret name
*/}}
{{- define "goatflow.appSecretName" -}}
{{- .Values.secrets.existingSecret | default (printf "%s-app" (include "goatflow.fullname" .)) }}
{{- end }}

{{/*
Secret value that survives upgrades: the user-supplied value, else the value stored in
the live Secret (lookup), else a newly generated one. Returns the base64-encoded value.
Usage: include "goatflow.secretValue" (dict "ctx" . "secret" <name> "key" <key> "value" <user value> "generate" <fresh value>)
*/}}
{{- define "goatflow.secretValue" -}}
{{- if .value }}
{{- .value | toString | b64enc }}
{{- else }}
{{- $existing := lookup "v1" "Secret" .ctx.Release.Namespace .secret }}
{{- if and $existing $existing.data (hasKey $existing.data .key) }}
{{- index $existing.data .key }}
{{- else }}
{{- .generate | b64enc }}
{{- end }}
{{- end }}
{{- end }}

{{/*
LDAP bind password secret name
*/}}
{{- define "goatflow.ldap.secretName" -}}
{{- .Values.config.ldap.existingSecret | default (printf "%s-ldap" (include "goatflow.fullname" .)) }}
{{- end }}

{{/*
Backend image
*/}}
{{- define "goatflow.backend.image" -}}
{{- $tag := .Values.backend.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.backend.image.repository $tag }}
{{- end }}

{{/*
Runner image
*/}}
{{- define "goatflow.runner.image" -}}
{{- $tag := .Values.runner.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.runner.image.repository $tag }}
{{- end }}

{{/*
Wait for the bundled database before GoatFlow starts: its headless Service has no DNS
record until the database pod is Ready, and GoatFlow connects only once at start.
*/}}
{{- define "goatflow.waitForDatabase" -}}
{{- if not .Values.database.external.enabled }}
initContainers:
  - name: wait-for-database
    image: {{ printf "%s:%s" .Values.waitForDatabase.image.repository .Values.waitForDatabase.image.tag | quote }}
    imagePullPolicy: {{ .Values.waitForDatabase.image.pullPolicy }}
    command:
      - sh
      - -c
      - until nc -z -w 2 {{ include "goatflow.database.host" . }} {{ include "goatflow.database.port" . }}; do echo "waiting for database"; sleep 2; done
    securityContext:
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
    resources:
      requests:
        cpu: 10m
        memory: 16Mi
      limits:
        cpu: 100m
        memory: 32Mi
{{- end }}
{{- end }}

{{/*
Environment shared by the backend and the runner. Names are the ones GoatFlow reads:
internal/platform/dbconfig (DB_DRIVER + DB_MYSQL_* / DB_PGSQL_*), config/default.yaml keys
through the GOATFLOW_ prefix, and plain os.Getenv names.
*/}}
{{- define "goatflow.commonEnv" -}}
{{- $prefix := include "goatflow.database.envPrefix" . }}
# Database: DB_DRIVER selects the DB_MYSQL_* or DB_PGSQL_* set
- name: DB_DRIVER
  value: {{ include "goatflow.database.driver" . | quote }}
- name: {{ $prefix }}HOST
  value: {{ include "goatflow.database.host" . | quote }}
- name: {{ $prefix }}PORT
  value: {{ include "goatflow.database.port" . | quote }}
- name: {{ $prefix }}NAME
  value: {{ include "goatflow.database.name" . | quote }}
- name: {{ $prefix }}USER
  valueFrom:
    secretKeyRef:
      name: {{ include "goatflow.database.secretName" . }}
      key: {{ eq .Values.database.type "mysql" | ternary "mysql-user" "postgres-user" }}
- name: {{ $prefix }}PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ include "goatflow.database.secretName" . }}
      key: {{ eq .Values.database.type "mysql" | ternary "mysql-password" "postgres-password" }}
{{- if or .Values.valkey.enabled .Values.externalValkey.enabled }}
# Cache (Valkey/Redis)
- name: GOATFLOW_VALKEY_HOST
  value: {{ include "goatflow.valkey.host" . | quote }}
- name: GOATFLOW_VALKEY_PORT
  value: {{ include "goatflow.valkey.port" . | quote }}
{{- with include "goatflow.valkey.passwordRef" . }}
{{- $ref := splitList "/" . }}
- name: GOATFLOW_VALKEY_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ index $ref 0 }}
      key: {{ index $ref 1 }}
{{- end }}
{{- end }}
# Application
- name: APP_ENV
  value: "production"
- name: LOG_LEVEL
  value: {{ .Values.config.logLevel | quote }}
- name: GOATFLOW_AUTH_SESSION_SESSIONMAXTIME
  value: {{ .Values.config.session.lifetime | quote }}
{{- with .Values.config.baseUrl }}
- name: BASE_URL
  value: {{ . | quote }}
{{- end }}
- name: STORAGE_TYPE
  value: {{ .Values.config.storage.type | quote }}
- name: STORAGE_PATH
  value: {{ .Values.config.storage.path | quote }}
# JWT signing key and the key that encrypts stored secrets (webhook signing secrets,
# plugin secure settings); the backend and the runner must share both
- name: JWT_SECRET
  valueFrom:
    secretKeyRef:
      name: {{ include "goatflow.appSecretName" . }}
      key: app-secret-key
- name: GOATFLOW_SECURE_KEY
  valueFrom:
    secretKeyRef:
      name: {{ include "goatflow.appSecretName" . }}
      key: secure-key
# Outgoing email
{{- $email := .Values.config.email }}
- name: GOATFLOW_EMAIL_ENABLED
  value: {{ $email.enabled | quote }}
{{- if $email.enabled }}
- name: GOATFLOW_EMAIL_FROM
  value: {{ $email.from | quote }}
{{- with $email.fromName }}
- name: GOATFLOW_EMAIL_FROM_NAME
  value: {{ . | quote }}
{{- end }}
- name: GOATFLOW_EMAIL_SMTP_HOST
  value: {{ required "config.email.smtp.host is required when config.email.enabled is true" $email.smtp.host | quote }}
- name: GOATFLOW_EMAIL_SMTP_PORT
  value: {{ $email.smtp.port | quote }}
- name: GOATFLOW_EMAIL_SMTP_TLS
  value: {{ $email.smtp.startTLS | quote }}
- name: GOATFLOW_EMAIL_SMTP_SKIP_VERIFY
  value: {{ $email.smtp.skipVerify | quote }}
- name: GOATFLOW_EMAIL_SMTP_AUTH_TYPE
  value: {{ $email.smtp.authType | quote }}
{{- with $email.smtp.username }}
- name: GOATFLOW_EMAIL_SMTP_USER
  value: {{ . | quote }}
{{- end }}
{{- if or $email.smtp.password $email.smtp.existingSecret }}
- name: GOATFLOW_EMAIL_SMTP_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ $email.smtp.existingSecret | default (include "goatflow.appSecretName" .) }}
      key: {{ ternary $email.smtp.existingSecretKey "smtp-password" (not (empty $email.smtp.existingSecret)) }}
{{- end }}
{{- end }}
{{- end }}
