/* eslint-disable space-infix-ops */
/* eslint-disable comma-spacing */
/* eslint-disable camelcase */
/* eslint-disable one-var */
/* eslint-disable no-multi-spaces */
const express = require('express')
const router  = express.Router()
// const pki = require('node-forge').pki
// const rsa = pki.rsa
const fs = require('fs')
const CERT_PATH = require('../config/certGen').CERT_PATH
const { spawnSync } = require('child_process')

/* GET home page. */
router.get('/', function (req, res, next) {
  res.render('index', {title: 'Beautiful App'})
})

// router.get('/crash', function (req, res, next) {
//   console.log(new Error('Requested crash by endpoint /crash'))
//   process.exit(1)
// })

// TODO: is not optimized and creates resource consumption peaks
// router.get('/generatecert', function (req, res, next) {
//   const keys = pki.rsa.generateKeyPair(2048)
//   const cert = pki.createCertificate()
//   cert.publicKey = keys.publicKey
//   res.send({
//     keys: keys,
//     cert: cert
//   })
// })

// Still too slow
// router.get('/generatecert', function (req, res, next) {
//   rsa.generateKeyPair({bits: 2048, workers: -1}, function (_err, keypair) {
//     const cert = pki.createCertificate()
//     cert.publicKey = keypair.publicKey
//     res.send({
//       keys: keypair,
//       cert: cert
//     })
//   })
// })

router.get('/generatecert', function (req, res, next) {
  const key  = fs.readFileSync(CERT_PATH + 'localhost.key')
  const cert = fs.readFileSync(CERT_PATH + 'localhost.cert')
  res.send({
    keys: key,
    cert: cert
  })
})

/**
 * This API will scale up or down the app based on the given parameter 'instances'
 * @param instances
 *
 * WIP / not working:
 *  my idea was to leverage a sibling docker to control the cluster also from here
 *  giving admin credentials
 * TODO: credentials
 */
// router.get('/scale/:instances', function (req, res, next) {
//   // REPLICAS=$( docker service ps phoenix_app | grep Running| wc -l)
//   // docker service update --replicas $(($REPLICAS + 1)) phoenix_app
//   var return_msg = ''
//   var docker_ver = spawnSync('docker -v', {shell: true})

//   if (docker_ver.stdout.length > 0) {
//     console.log('> ' + docker_ver.stdout.toString())

//     var replicas = spawnSync('docker service ps phoenix_app | grep Running| wc -l', {shell: true})

//     if (replicas.stdout.length > 0 && Number(replicas.stdout.toString()) > 0) {
//       replicas = replicas.stdout.toString().trim()
//       console.log('> There are already ' + replicas + ' instances running')

//       let num_of_instances = Number(req.params.instances)
//       let tot_instances    = Number(replicas) + num_of_instances

//       var update_cmd = 'docker service update --replicas ' + tot_instances + ' phoenix_app'
//       console.log('> Executing: ' + update_cmd)
//       var service_update = spawnSync(update_cmd, {shell: true})

//       if (service_update.stdout != null && service_update.stdout.length > 0) {
//         console.log('> scale/'+ num_of_instances +' stdout: ' + service_update.stdout.toString())
//       } else {
//         console.log('> stderr: docker service returned an error or 0 instances.\n' + service_update.stderr.toString())
//       }
//       return_msg = '> Service scaled of '+num_of_instances+' instances.'

//     // Something went wrong starting docker
//     } else {
//       console.log('> Docker Error: ' + replicas.stderr.toString())
//       res.send()
//     }
//   } else {
//     console.log('> Error: ' + docker_ver.stderr.toString())
//   }
//   return_msg = 'There was a problem scaling the service!'

//   res.send(return_msg)
// })

module.exports = router
