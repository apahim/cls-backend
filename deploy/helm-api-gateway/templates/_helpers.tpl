{{/*
Expand the name of the chart.
*/}}
{{- define "cls-backend-api-gateway.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "cls-backend-api-gateway.fullname" -}}
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
{{- define "cls-backend-api-gateway.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "cls-backend-api-gateway.labels" -}}
helm.sh/chart: {{ include "cls-backend-api-gateway.chart" . }}
{{ include "cls-backend-api-gateway.selectorLabels" . }}
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
{{- define "cls-backend-api-gateway.selectorLabels" -}}
app.kubernetes.io/name: {{ include "cls-backend-api-gateway.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Generate backend address for API Gateway
*/}}
{{- define "cls-backend-api-gateway.backendAddress" -}}
{{- $externalIP := include "cls-backend-api-gateway.getExternalIP" . -}}
{{- printf "%s://%s.nip.io:%v" .Values.backend.protocol $externalIP (.Values.backend.port | int) }}
{{- end }}

{{/*
Get external IP from LoadBalancer service or fallback to manual value
*/}}
{{- define "cls-backend-api-gateway.getExternalIP" -}}
{{- $manualIP := .Values.backend.externalIP -}}
{{- $namespace := .Values.backend.namespace | default "cls-system" -}}
{{- $serviceName := .Values.backend.serviceName | default "cls-application-external" -}}

{{- /* Try to lookup the LoadBalancer service */ -}}
{{- $service := lookup "v1" "Service" $namespace $serviceName -}}
{{- if and $service $service.status.loadBalancer.ingress -}}
{{- $autoIP := (index $service.status.loadBalancer.ingress 0).ip -}}
{{- if $autoIP -}}
{{- $autoIP -}}
{{- else -}}
{{- if not $manualIP -}}
{{- fail (printf "\n❌ LoadBalancer service '%s/%s' exists but has no external IP yet.\n\n📋 WAIT AND RETRY:\n   kubectl get service %s -n %s --watch\n\n⏱️  LoadBalancers can take 2-5 minutes to get an IP address." $namespace $serviceName $serviceName $namespace) -}}
{{- end -}}
{{- $manualIP -}}
{{- end -}}
{{- else -}}
{{- if not $manualIP -}}
{{- fail (printf "\n❌ Cannot find LoadBalancer service '%s/%s' and no manual backend.externalIP provided.\n\n📋 REQUIRED ACTION:\n   Either:\n   1. Deploy the application chart first: helm install cls-application ./helm-application\n   2. Or set backend.externalIP manually in values.yaml\n\n🔗 The LoadBalancer service should be created by the application chart." $namespace $serviceName) -}}
{{- end -}}
{{- $manualIP -}}
{{- end -}}
{{- end }}

{{/*
Generate API Config ID with timestamp
*/}}
{{- define "cls-backend-api-gateway.apiConfigId" -}}
{{- $timestamp := now | date "20060102-150405" }}
{{- printf "%s-%s" .Values.apiGateway.apiConfig.idPrefix $timestamp }}
{{- end }}

{{/*
Generate API Config display name with timestamp
*/}}
{{- define "cls-backend-api-gateway.apiConfigDisplayName" -}}
{{- $timestamp := now | date "2006-01-02 15:04" }}
{{- printf "%s - %s" .Values.apiGateway.apiConfig.displayNamePrefix $timestamp }}
{{- end }}

{{/*
Generate CORS origins as YAML array
*/}}
{{- define "cls-backend-api-gateway.corsOrigins" -}}
{{- range $index, $origin := .Values.cors.allowedOrigins }}
{{- if $index }}
{{- printf "\n  " }}
{{- end }}
{{- printf "- origin: %q" $origin }}
{{- printf "\n    methods: %s" (toJson $.Values.cors.allowedMethods) }}
{{- printf "\n    headers: %s" (toJson $.Values.cors.allowedHeaders) }}
{{- printf "\n    maxAge: %d" ($.Values.cors.maxAge | int) }}
{{- printf "\n    allowCredentials: %t" $.Values.cors.allowCredentials }}
{{- end }}
{{- end }}

{{/*
Validate backend external IP format
*/}}
{{- define "cls-backend-api-gateway.validateBackendIP" -}}
{{- $ip := .Values.backend.externalIP -}}
{{- if $ip -}}
{{- if not (regexMatch "^(?:[0-9]{1,3}\\.){3}[0-9]{1,3}$" $ip) -}}
{{- fail (printf "backend.externalIP '%s' is not a valid IPv4 address format. Please provide a valid static IP address (e.g., 34.172.156.70)" $ip) -}}
{{- end -}}
{{- if or (hasPrefix "0." $ip) (hasPrefix "127." $ip) (hasPrefix "10." $ip) (hasPrefix "172." $ip) (hasPrefix "192.168." $ip) -}}
{{- fail (printf "backend.externalIP '%s' appears to be a private/localhost IP. Please provide a valid public static IP address that matches your LoadBalancer" $ip) -}}
{{- end -}}
{{- else -}}
{{- fail "backend.externalIP is required. Please provide the static IP address of your external LoadBalancer service" -}}
{{- end -}}
{{- end }}

{{/*
Cross-chart parameter validation for consistency with cloud-resources and application charts
*/}}
{{- define "cls-backend-api-gateway.validateCrossChartParams" -}}
{{- /* Validate required parameters */ -}}
{{- if not .Values.gcp.project -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: gcp.project\n\n📋 REQUIRED ACTION:\n   Set gcp.project to your Google Cloud Project ID\n\n💡 EXAMPLE:\n   gcp:\n     project: \"my-gcp-project-123\"\n\n⚠️  IMPORTANT: This value must match exactly in all three charts:\n   - helm-cloud-resources/values.yaml\n   - helm-application/values.yaml  \n   - helm-api-gateway/values.yaml\n\n🔗 More info: https://cloud.google.com/resource-manager/docs/creating-managing-projects" -}}
{{- end -}}

{{- /* Note: backend.externalIP is now auto-discovered from LoadBalancer service or can be set manually */ -}}

{{- /* Validate API Gateway configuration */ -}}
{{- if not .Values.apiGateway.api.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: apiGateway.api.name\n\n📋 REQUIRED ACTION:\n   Set apiGateway.api.name for the Google API Gateway API resource\n\n💡 EXAMPLE:\n   apiGateway:\n     api:\n       name: \"cls-backend-api\"\n       displayName: \"CLS Backend API\"\n\n🔗 More info: This creates the API Gateway API resource in Google Cloud" -}}
{{- end -}}

{{- if not .Values.apiGateway.gateway.name -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: apiGateway.gateway.name\n\n📋 REQUIRED ACTION:\n   Set apiGateway.gateway.name for the Google API Gateway Gateway resource\n\n💡 EXAMPLE:\n   apiGateway:\n     gateway:\n       name: \"cls-backend-gateway\"\n       displayName: \"CLS Backend Gateway\"\n\n🔗 More info: This creates the API Gateway Gateway resource with public endpoint" -}}
{{- end -}}

{{- /* Validate OAuth2 configuration */ -}}
{{- if not .Values.oauth2.clientId -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: oauth2.clientId\n\n📋 REQUIRED ACTION:\n   Set oauth2.clientId for Google OAuth2 authentication\n\n💡 EXAMPLE:\n   oauth2:\n     clientId: \"123456789.apps.googleusercontent.com\"\n     scopes:\n       - \"openid\"\n       - \"email\"\n       - \"profile\"\n\n🔗 More info: Create OAuth2 credentials in Google Cloud Console\n\n📖 HOW TO CREATE OAUTH2 CLIENT:\n   1. Go to Google Cloud Console > APIs & Services > Credentials\n   2. Create OAuth 2.0 Client ID\n   3. Add your gateway domain to authorized origins\n   4. Use the generated Client ID here" -}}
{{- end -}}

{{- /* Validate CORS configuration */ -}}
{{- if not .Values.cors.allowedOrigins -}}
{{- fail "\n❌ MISSING REQUIRED VALUE: cors.allowedOrigins\n\n📋 REQUIRED ACTION:\n   Set cors.allowedOrigins for frontend access control\n\n💡 EXAMPLE:\n   cors:\n     allowedOrigins:\n       - \"https://console.redhat.com\"\n       - \"https://hybrid-cloud-console.redhat.com\"\n     allowedMethods:\n       - \"GET\"\n       - \"POST\"\n       - \"PUT\"\n       - \"DELETE\"\n\n🔗 More info: List all frontend domains that should be allowed to access the API" -}}
{{- end -}}

{{- /* Cross-chart consistency warnings */ -}}
{{- $backendPort := .Values.backend.port -}}
{{- $backendProtocol := .Values.backend.protocol -}}

{{- if ne $backendPort 80 -}}
{{- printf "\nWARNING: backend.port='%v' should typically be 80 to match externalService.port in the application chart" $backendPort | fail -}}
{{- end -}}

{{- if ne $backendProtocol "http" -}}
{{- printf "\nWARNING: backend.protocol='%s' should typically be 'http' for LoadBalancer backends" $backendProtocol | fail -}}
{{- end -}}

{{- /* Validate specific OAuth2 configuration for typical deployment */ -}}
{{- $expectedClientId := "32555940559.apps.googleusercontent.com" -}}
{{- if ne .Values.oauth2.clientId $expectedClientId -}}
{{- printf "\nNOTE: oauth2.clientId='%s' differs from the default. Ensure this matches your Google Cloud Console OAuth2 configuration" .Values.oauth2.clientId -}}
{{- end -}}

{{- /* Validate CORS origins for typical Red Hat Console integration */ -}}
{{- $expectedOrigins := list "https://console.redhat.com" "https://hybrid-cloud-console.redhat.com" -}}
{{- $hasConsoleOrigin := false -}}
{{- range .Values.cors.allowedOrigins -}}
{{- if or (eq . "https://console.redhat.com") (eq . "https://hybrid-cloud-console.redhat.com") -}}
{{- $hasConsoleOrigin = true -}}
{{- end -}}
{{- end -}}
{{- if not $hasConsoleOrigin -}}
{{- printf "\nNOTE: cors.allowedOrigins does not include Red Hat Console origins. Consider adding https://console.redhat.com and https://hybrid-cloud-console.redhat.com" -}}
{{- end -}}
{{- end }}