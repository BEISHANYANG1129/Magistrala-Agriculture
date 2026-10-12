param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('ui', 'image', 'control', 'llm', 'executor', 'integration', 'agent-ui')]
    [string]$Service
)
$ErrorActionPreference = 'Stop'
& node (Join-Path $PSScriptRoot 'scripts/run-local.mjs') $Service
exit $LASTEXITCODE
