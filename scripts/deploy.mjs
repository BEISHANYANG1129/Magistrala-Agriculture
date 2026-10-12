import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { fileURLToPath } from 'node:url'
import { spawnSync } from 'node:child_process'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dockerDir = path.join(root, 'magistrala', 'docker')
const action = process.argv[2] || 'status'
function run(command, args, options = {}) {
  const result = spawnSync(command, args, { cwd: root, stdio: 'inherit', ...options })
  if (result.error) throw result.error
  if (result.status !== 0) throw new Error(`${command} failed (exit ${result.status})`)
}
function writeOnce(filename, text, mode = 0o600) {
  if (!fs.existsSync(filename)) {
    fs.mkdirSync(path.dirname(filename), { recursive: true })
    fs.writeFileSync(filename, text, { mode, flag: 'wx' })
  }
}
function initialize() {
  let template = fs.readFileSync(path.join(dockerDir, '.env.example'), 'utf8')
  template = template.replace(/__GENERATE_(HEX_32|HEX_16|BASE64_32)__/g, (_, kind) =>
    crypto.randomBytes(kind === 'HEX_16' ? 16 : 32).toString(kind === 'BASE64_32' ? 'base64' : 'hex'))
  writeOnce(path.join(dockerDir, '.env'), template)
  const env = Object.fromEntries(fs.readFileSync(path.join(dockerDir, '.env'), 'utf8').split(/\r?\n/)
    .filter(s => /^[A-Z][A-Z0-9_]*=/.test(s)).map(s => [s.slice(0,s.indexOf('=')),s.slice(s.indexOf('=')+1)]))
  const bootstrap = path.join(dockerDir, 'atom-bootstrap.yaml')
  const tokens = path.join(dockerDir, '.env.tokens')
  if (fs.existsSync(bootstrap) !== fs.existsSync(tokens)) throw new Error('Incomplete token pair: restore both atom-bootstrap.yaml and .env.tokens from the same backup.')
  if (!fs.existsSync(tokens)) {
    const names = ['FLUXMQ_AUTH','FLUXMQ_NODE1','FLUXMQ_NODE2','FLUXMQ_NODE3','JOURNAL','NOTIFICATIONS','RE','ALARMS','REPORTS','TIMESCALE_READER','POSTGRES_READER','BOOTSTRAP']
    let yaml = fs.readFileSync(path.join(dockerDir, 'atom-bootstrap.template.yaml'),'utf8')
    yaml += '\nentities:\n  - id: 00000000-0000-0000-0000-000000000003\n    kind: service\n    name: example-service\n    attributes:\n      system: true\n      purpose: example-service-integration\n    credentials:\n'
    const entries = names.map(name => {
      const token = `atom_${crypto.randomBytes(16).toString('hex')}_${crypto.randomBytes(32).toString('hex')}`
      yaml += `      - kind: access_token\n        token: ${token}\n        name: ${name.toLowerCase().replaceAll('_','-')}\n`
      return `MG_ATOM_TOKEN_${name}=${token}`
    })
    // Atom is non-root and reads this bind mount in the development stack.
    writeOnce(bootstrap,yaml,0o644)
    writeOnce(tokens,'# Generated locally; do not commit.\n'+entries.join('\n')+'\n')
  }
  const brokers = { RE:'re', TIMESCALE_WRITER:'timescale-writer', POSTGRES_WRITER:'postgres-writer', FLUXMQ:'fluxmq-auth', ALARMS:'alarms' }
  for (const [key,name] of Object.entries(brokers)) {
    const secret = env[`MG_${key}_BROKER_SECRET`]
    if (!secret || secret.length < 32) throw new Error(`MG_${key}_BROKER_SECRET must contain at least 32 characters`)
    writeOnce(path.join(dockerDir,'fluxmq','secrets',`${name}-current`),secret,0o644)
  }
  writeOnce(path.join(dockerDir,'re','secrets','trace.key'),crypto.randomBytes(32).toString('hex')+'\n',0o644)
  for (const name of ['data/magistrala','data/device_registry','agriDataIntegration/config','agriDataIntegration/sensor_mapping','agriDeviceExecutor/internal/config/config','agriDeviceExecutor/internal/data/executor_mapping','llm/config/config']) {
    writeOnce(path.join(root,name+'.json'),fs.readFileSync(path.join(root,name+'.example.json'),'utf8'))
  }
  const certDir = path.join(dockerDir,'ssl','certs')
  fs.mkdirSync(certDir,{recursive:true})
  if (!fs.existsSync(path.join(certDir,'ca.key'))) {
    const probe=spawnSync('openssl',['version'],{stdio:'ignore'})
    if (probe.status === 0) generateCertificates(certDir)
    else run('docker',['run','--rm','-v',`${root}:/work`,'-w','/work','alpine:3.24','sh','-c','apk add --no-cache nodejs openssl >/dev/null && node scripts/deploy.mjs certificates'])
  }
  for (const file of ['ca.crt','ca.key','magistrala-server.crt','magistrala-server.key','fluxmq-service-server.crt','fluxmq-service-server.key',...Object.values(brokers).flatMap(name=>[`${name}-fluxmq-client.crt`,`${name}-fluxmq-client.key`])]) {
    if (!fs.existsSync(path.join(certDir,file))) throw new Error(`Certificate missing: ${file}; repair certificate set before starting.`)
  }
  console.log('Initialized local config, service tokens, broker secrets and TLS certificates. Existing files were preserved.')
  console.log('Administrator username: admin. Password: ATOM_ADMIN_SECRET in magistrala/docker/.env (not printed).')
}
function generateCertificates(directory) {
  const openssl = args => run('openssl',args,{cwd:directory,stdio:'pipe'})
  fs.writeFileSync(path.join(directory,'request.conf'),'[req]\ndistinguished_name=dn\nprompt=no\n[dn]\nCN=Magistrala\n')
  openssl(['req','-config','request.conf','-newkey','rsa:2048','-x509','-nodes','-sha256','-days','1095','-keyout','ca.key','-out','ca.crt','-subj','/CN=Magistrala-Agriculture-Local-CA','-addext','basicConstraints=critical,CA:TRUE','-addext','keyUsage=critical,keyCertSign,cRLSign'])
  const certificates = [
    ['magistrala-server','localhost','serverAuth','DNS:localhost,DNS:nginx,IP:127.0.0.1'],
    ['fluxmq-service-server','fluxmq','serverAuth','DNS:fluxmq,DNS:fluxmq-node1,DNS:fluxmq-node2,DNS:fluxmq-node3'],
    ...['re','timescale-writer','postgres-writer','fluxmq-auth','alarms'].map(name=>[`${name}-fluxmq-client`,name,'clientAuth',`URI:spiffe://absmach/magistrala/${name==='re'?'rules-engine':name}`]),
  ]
  for (const [name,cn,usage,san] of certificates) {
    fs.writeFileSync(path.join(directory,`${name}.conf`),`[v3_req]\nsubjectAltName=${san}\nextendedKeyUsage=${usage}\n`)
    openssl(['req','-config','request.conf','-new','-newkey','rsa:2048','-nodes','-sha256','-keyout',`${name}.key`,'-out',`${name}.csr`,'-subj',`/CN=${cn}`])
    openssl(['x509','-req','-sha256','-days','365','-in',`${name}.csr`,'-CA','ca.crt','-CAkey','ca.key','-CAcreateserial','-out',`${name}.crt`,'-extfile',`${name}.conf`,'-extensions','v3_req'])
    fs.unlinkSync(path.join(directory,`${name}.csr`)); fs.unlinkSync(path.join(directory,`${name}.conf`))
  }
  // Development bind mounts must be readable by non-root container users.
  for (const file of fs.readdirSync(directory)) fs.chmodSync(path.join(directory,file),0o644)
}
const composeArgs=['compose','-f',path.join(dockerDir,'docker-compose.yaml'),'-f',path.join(dockerDir,'compose.pinned.json'),'-f',path.join(dockerDir,'compose.agriculture.yaml'),'--env-file',path.join(dockerDir,'.env'),'--env-file',path.join(dockerDir,'.env.tokens')]
try {
  switch(action) {
    case 'init': initialize(); break
    case 'certificates': generateCertificates(path.join(dockerDir,'ssl','certs')); break
    case 'up':
      initialize()
      run('docker',[...composeArgs,'build','ui','agriculture'])
      // Bring up the broker before enabling Atom's event publisher, avoiding its cold-start cycle.
      run('docker',[...composeArgs,'up','-d','nginx'],{env:{...process.env,ATOM_EVENTS_AMQP_URL:''}})
      run('docker',[...composeArgs,'up','-d']); break
    case 'down': run('docker',[...composeArgs,'down']); break
    case 'status': run('docker',[...composeArgs,'ps']); break
    case 'config': run('docker',[...composeArgs,'config','--quiet']); break
    case 'logs': run('docker',[...composeArgs,'logs','--tail','100',...process.argv.slice(3)]); break
    default: throw new Error('Usage: node scripts/deploy.mjs init|up|down|status|config|logs [service]')
  }
} catch(error) { console.error(error.message); process.exitCode=1 }
