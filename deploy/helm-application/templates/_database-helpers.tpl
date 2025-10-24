{{/*
Database connection name helper
Generates the Cloud SQL connection name in the correct format: PROJECT:REGION:INSTANCE
*/}}
{{- define "cls-backend-application.cloudSqlConnectionName" -}}
{{- if .Values.database.cloudSql.connectionName -}}
{{- .Values.database.cloudSql.connectionName -}}
{{- else if and .Values.gcp.project .Values.database.instanceName -}}
{{- printf "%s:%s:%s" .Values.gcp.project (.Values.gcp.region | default "us-central1") .Values.database.instanceName -}}
{{- else -}}
{{- fail "database.cloudSql.connectionName is required when using Cloud SQL Proxy, or set gcp.project and database.instanceName" -}}
{{- end -}}
{{- end -}}

{{/*
Database configuration validation
*/}}
{{- define "cls-backend-application.validateDatabaseConfig" -}}
{{- if eq .Values.database.type "cloud-sql" -}}
  {{- if .Values.database.cloudSql.enableProxy -}}
    {{- if not (or .Values.database.cloudSql.connectionName (and .Values.gcp.project .Values.database.instanceName)) -}}
      {{- fail "When using Cloud SQL Proxy, either database.cloudSql.connectionName or both gcp.project and database.instanceName must be set" -}}
    {{- end -}}
  {{- else -}}
    {{- if not .Values.database.instanceName -}}
      {{- fail "When using Cloud SQL without proxy, database.instanceName is required" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
{{- end -}}