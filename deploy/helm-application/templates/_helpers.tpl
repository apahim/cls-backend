{{/*
Expand the name of the chart.
*/}}
{{- define "cls-backend-application.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cls-backend-application.fullname" -}}
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
{{- define "cls-backend-application.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cls-backend-application.labels" -}}
helm.sh/chart: {{ include "cls-backend-application.chart" . }}
{{ include "cls-backend-application.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "cls-backend-application.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cls-backend-application.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Auto-discover database instance name from cloud-resources chart
*/}}
{{- define "cls-backend-application.getDatabaseInstanceName" -}}
{{- $manualInstanceName := .Values.database.instanceName -}}

{{- /* Try to lookup SQLInstance from Config Connector */ -}}
{{- if not $manualInstanceName -}}
{{- $sqlInstances := lookup "sql.cnrm.cloud.google.com/v1beta1" "SQLInstance" "config-connector" "" -}}
{{- if $sqlInstances.items -}}
{{- range $sqlInstances.items -}}
{{- if hasPrefix "cls-backend" .metadata.name -}}
{{- .metadata.name -}}
{{- break -}}
{{- end -}}
{{- end -}}
{{- else -}}
{{- "cls-backend-db" -}}
{{- end -}}
{{- else -}}
{{- $manualInstanceName -}}
{{- end -}}
{{- end }}

{{/*
Auto-discover database name from cloud-resources chart
*/}}
{{- define "cls-backend-application.getDatabaseName" -}}
{{- $manualDatabaseName := .Values.database.databaseName -}}
{{- if $manualDatabaseName -}}
{{- $manualDatabaseName -}}
{{- else -}}
{{- "cls_backend" -}}
{{- end -}}
{{- end }}

{{/*
Auto-discover database username from cloud-resources chart
*/}}
{{- define "cls-backend-application.getDatabaseUsername" -}}
{{- $manualUsername := .Values.database.username -}}
{{- if $manualUsername -}}
{{- $manualUsername -}}
{{- else -}}
{{- "cls_user" -}}
{{- end -}}
{{- end }}

{{/*
Auto-discover GCP service account from cloud-resources chart
*/}}
{{- define "cls-backend-application.getServiceAccount" -}}
{{- $manualServiceAccount := .Values.serviceAccount.gcpServiceAccountName -}}

{{- /* Try to lookup IAMServiceAccount from Config Connector */ -}}
{{- $serviceAccounts := lookup "iam.cnrm.cloud.google.com/v1beta1" "IAMServiceAccount" "config-connector" "" -}}
{{- if $serviceAccounts.items -}}
{{- range $serviceAccounts.items -}}
{{- if and (hasPrefix "cls-backend" .metadata.name) (not $manualServiceAccount) -}}
{{- .metadata.name -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- /* Return manual value if provided */ -}}
{{- if $manualServiceAccount -}}
{{- $manualServiceAccount -}}
{{- else -}}
{{- "cls-backend" -}}
{{- end -}}
{{- end }}

{{/*
Auto-discover Pub/Sub topic from cloud-resources chart
*/}}
{{- define "cls-backend-application.getPubSubTopic" -}}
{{- $manualTopic := .Values.pubsub.clusterEventsTopic -}}

{{- /* Try to lookup PubSubTopic from Config Connector */ -}}
{{- $topics := lookup "pubsub.cnrm.cloud.google.com/v1beta1" "PubSubTopic" "config-connector" "" -}}
{{- if $topics.items -}}
{{- range $topics.items -}}
{{- if and (contains "cluster-events" .metadata.name) (not $manualTopic) -}}
{{- .metadata.name -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- /* Return manual value if provided */ -}}
{{- if $manualTopic -}}
{{- $manualTopic -}}
{{- else -}}
{{- "cluster-events" -}}
{{- end -}}
{{- end }}

{{/*
Auto-discover Secret Manager secret from cloud-resources chart
*/}}
{{- define "cls-backend-application.getPasswordSecret" -}}
{{- $manualSecret := .Values.database.passwordSecret.name -}}

{{- /* Try to lookup SecretManagerSecret from Config Connector */ -}}
{{- $secrets := lookup "secretmanager.cnrm.cloud.google.com/v1beta1" "SecretManagerSecret" "config-connector" "" -}}
{{- if $secrets.items -}}
{{- range $secrets.items -}}
{{- if and (contains "password" .metadata.name) (hasPrefix "cls-backend" .metadata.name) (not $manualSecret) -}}
{{- .metadata.name -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- /* Return manual value if provided */ -}}
{{- if $manualSecret -}}
{{- $manualSecret -}}
{{- else -}}
{{- "cls-backend-db-password" -}}
{{- end -}}
{{- end }}

{{/*
Cross-chart parameter validation for consistency with cloud-resources and API gateway charts
*/}}
{{- define "cls-backend-application.validateCrossChartParams" -}}
{{- /* Validate required parameters */ -}}
{{- if not .Values.gcp.project -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: gcp.project\n\n📋 REQUIRED ACTION:\n   Set gcp.project to your Google Cloud Project ID\n\n💡 EXAMPLE:\n   gcp:\n     project: \"my-gcp-project-123\"\n\n⚠️  IMPORTANT: This value must match exactly in all three charts:\n   - helm-cloud-resources/values.yaml\n   - helm-application/values.yaml  \n   - helm-api-gateway/values.yaml\n\n🔗 More info: https://cloud.google.com/resource-manager/docs/creating-managing-projects" -}}
{{- end -}}

{{- /* Note: Database, Pub/Sub, and Service Account configuration is now auto-discovered from cloud-resources chart */ -}}
{{- /* Manual validation removed - values are auto-discovered using Helm lookup from Config Connector CRDs */ -}}

{{- /* Validate namespace consistency */ -}}
{{- if not .Values.namespace.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: namespace.name\n\n📋 REQUIRED ACTION:\n   Set namespace.name for the Kubernetes namespace\n\n💡 EXAMPLE:\n   namespace:\n     name: \"cls-system\"\n     create: true\n\n⚠️  IMPORTANT: This value must match workloadIdentity.kubernetesNamespace in the cloud-resources chart\n\n🔗 More info: This is where all application resources will be deployed" -}}
{{- end -}}

{{- /* Cross-chart consistency is now handled automatically via auto-discovery */ -}}
{{- end }}