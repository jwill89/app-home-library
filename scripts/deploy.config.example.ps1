# Copy to deploy.config.ps1 (git-ignored) and fill in. deploy.ps1 dot-sources it for any
# setting not passed as a parameter or set as DEPLOY_VPS_HOST, DEPLOY_SERVER_NAME and so on.
# Nothing here is the repository's to know: the host, the key and the layout stay on this
# machine.
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseDeclaredVarsMoreThanAssignments', '', Justification = 'deploy.ps1 dot-sources this file and reads these')]
param()

$DeployVpsHost = '<droplet-ip-or-hostname>'
$DeployVpsUser = '<ssh-user>'
$DeployKeyPath = 'C:\path\to\deploy-key.ppk'
# The web server's systemd service (the text service is <service-name>-text, and the vhost file
# is /etc/apache2/sites-available/<service-name>.conf), and the account both run as.
$DeployServiceName = '<service-name>'
$DeployServiceUser = '<service-name>'
# The folder on the host that holds the binary, with data/ and backups/ inside it.
$DeployRemoteOptDir = '/opt/<service-name>'
# The site's main host name, its other names (space-separated; leave out for none), and the
# folder of the certificate under /etc/letsencrypt/live that covers them all.
$DeployServerName = '<site-host-name>'
$DeployServerAlias = '<other-host-name>'
$DeployCertName = '<certificate-folder>'
# The loopback port the web server listens on: the port in HL_ADDR in its env file. Each site
# on the droplet has its own.
$DeployAppPort = '<port>'
# Optional: the services' env file, if not /etc/<service-name>/<service-name>.env.
# $DeployEnvFile = '/etc/<service-name>/<service-name>.env'
# Optional: the address the smoke check fetches, if not https:// and the ServerName.
# $DeploySiteUrl = 'https://<site-host-name>'
