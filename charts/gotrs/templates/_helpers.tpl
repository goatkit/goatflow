{{/*
Expand the name of the chart.
*/}}
{{- define "gotrs.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gotrs.fullname" -}}
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
{{- define "gotrs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gotrs.labels" -}}
helm.sh/chart: {{ include "gotrs.chart" . }}
{{ include "gotrs.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gotrs.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gotrs.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Backend labels
*/}}
{{- define "gotrs.backend.labels" -}}
{{ include "gotrs.labels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Backend selector labels
*/}}
{{- define "gotrs.backend.selectorLabels" -}}
{{ include "gotrs.selectorLabels" . }}
app.kubernetes.io/component: backend
{{- end }}

{{/*
Frontend labels
*/}}
{{- define "gotrs.frontend.labels" -}}
{{ include "gotrs.labels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Frontend selector labels
*/}}
{{- define "gotrs.frontend.selectorLabels" -}}
{{ include "gotrs.selectorLabels" . }}
app.kubernetes.io/component: frontend
{{- end }}

{{/*
Database labels
*/}}
{{- define "gotrs.database.labels" -}}
{{ include "gotrs.labels" . }}
app.kubernetes.io/component: database
{{- end }}

{{/*
Database selector labels
*/}}
{{- define "gotrs.database.selectorLabels" -}}
{{ include "gotrs.selectorLabels" . }}
app.kubernetes.io/component: database
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gotrs.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "gotrs.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the appropriate storage class
*/}}
{{- define "gotrs.storageClass" -}}
{{- $storageClass := .Values.global.storageClass -}}
{{- if $storageClass -}}
storageClassName: {{ $storageClass | quote }}
{{- end -}}
{{- end }}

{{/*
Database host
*/}}
{{- define "gotrs.database.host" -}}
{{- if .Values.database.external.enabled }}
{{- .Values.database.external.host }}
{{- else }}
{{- printf "%s-database" (include "gotrs.fullname" .) }}
{{- end }}
{{- end }}

{{/*
Database port
*/}}
{{- define "gotrs.database.port" -}}
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
{{- define "gotrs.database.name" -}}
{{- if .Values.database.external.enabled }}
{{- .Values.database.external.database }}
{{- else if eq .Values.database.type "mysql" }}
{{- .Values.database.mysql.database }}
{{- else }}
{{- .Values.database.postgresql.database }}
{{- end }}
{{- end }}

{{/*
Database user
*/}}
{{- define "gotrs.database.user" -}}
{{- if .Values.database.external.enabled }}
{{- "external" }}
{{- else if eq .Values.database.type "mysql" }}
{{- .Values.database.mysql.user }}
{{- else }}
{{- .Values.database.postgresql.user }}
{{- end }}
{{- end }}

{{/*
Database secret name
*/}}
{{- define "gotrs.database.secretName" -}}
{{- if .Values.database.external.enabled }}
{{- .Values.database.external.existingSecret | default (printf "%s-database-external" (include "gotrs.fullname" .)) }}
{{- else if eq .Values.database.type "mysql" }}
{{- .Values.database.mysql.existingSecret | default (printf "%s-database" (include "gotrs.fullname" .)) }}
{{- else }}
{{- .Values.database.postgresql.existingSecret | default (printf "%s-database" (include "gotrs.fullname" .)) }}
{{- end }}
{{- end }}

{{/*
Valkey/Redis host
*/}}
{{- define "gotrs.valkey.host" -}}
{{- if .Values.externalValkey.enabled }}
{{- .Values.externalValkey.host }}
{{- else if .Values.valkey.enabled }}
{{- printf "%s-valkey-master" .Release.Name }}
{{- end }}
{{- end }}

{{/*
Valkey/Redis port
*/}}
{{- define "gotrs.valkey.port" -}}
{{- if .Values.externalValkey.enabled }}
{{- .Values.externalValkey.port | default "6379" }}
{{- else }}
{{- "6379" }}
{{- end }}
{{- end }}

{{/*
Valkey secret name
*/}}
{{- define "gotrs.valkey.secretName" -}}
{{- if .Values.externalValkey.enabled }}
{{- .Values.externalValkey.existingSecret | default (printf "%s-valkey-external" (include "gotrs.fullname" .)) }}
{{- else if .Values.valkey.auth.existingSecret }}
{{- .Values.valkey.auth.existingSecret }}
{{- else }}
{{- printf "%s-valkey" .Release.Name }}
{{- end }}
{{- end }}

{{/*
App secret name
*/}}
{{- define "gotrs.appSecretName" -}}
{{- printf "%s-app" (include "gotrs.fullname" .) }}
{{- end }}

{{/*
Backend image
*/}}
{{- define "gotrs.backend.image" -}}
{{- $tag := .Values.backend.image.tag | default .Chart.AppVersion -}}
{{- printf "%s:%s" .Values.backend.image.repository $tag }}
{{- end }}

{{/*
Check if we should create database
*/}}
{{- define "gotrs.database.create" -}}
{{- if and (not .Values.database.external.enabled) (or (eq .Values.database.type "mysql") (eq .Values.database.type "postgresql")) }}
{{- true }}
{{- end }}
{{- end }}

{{/*
Common annotations - merges global.commonAnnotations with component-specific annotations
Usage: {{ include "gotrs.annotations" (dict "annotations" .Values.backend.podAnnotations "context" .) }}
*/}}
{{- define "gotrs.annotations" -}}
{{- $common := .context.Values.global.commonAnnotations | default dict -}}
{{- $specific := .annotations | default dict -}}
{{- $merged := merge $specific $common -}}
{{- if $merged }}
{{- toYaml $merged }}
{{- end }}
{{- end }}

{{/*
Common labels - merges global.commonLabels with standard labels and component-specific labels
Usage: {{ include "gotrs.mergedLabels" (dict "labels" .Values.backend.podLabels "context" .) }}
*/}}
{{- define "gotrs.mergedLabels" -}}
{{- $common := .context.Values.global.commonLabels | default dict -}}
{{- $specific := .labels | default dict -}}
{{- $merged := merge $specific $common -}}
{{- if $merged }}
{{- toYaml $merged }}
{{- end }}
{{- end }}
