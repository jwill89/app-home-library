# Copy to deploy.config.ps1 (git-ignored) and fill in. deploy.ps1 dot-sources it
# for any setting not passed as a parameter or set as DEPLOY_VPS_HOST,
# DEPLOY_VPS_USER, or DEPLOY_KEY_PATH.
[Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseDeclaredVarsMoreThanAssignments', '', Justification = 'deploy.ps1 dot-sources this file and reads the three')]
param()

$DeployVpsHost = '<droplet-ip-or-hostname>'
$DeployVpsUser = 'root'
$DeployKeyPath = 'C:\path\to\digitalocean-key.ppk'
