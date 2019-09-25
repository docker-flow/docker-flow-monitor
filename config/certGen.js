/* eslint-disable no-multi-spaces */
const exec = require('child_process').exec
const CERT_PATH = './cert/'
const KEY_SIZE  = 2048
const EXPIRATION_DAYS = 3650

function certGen () {
  exec('openssl genrsa -out ' + CERT_PATH + 'localhost.key ' + KEY_SIZE,
    {shell: '/bin/sh'},
    (err, stdout, stderr) => {
      if (err) {
        console.log((new Date()).toISOString() + '> X.509 keys generation fail!\n ' + err, stderr)
      } else {
        console.log((new Date()).toISOString() + '> X.509 keys have been generated. ', stdout)

        exec('openssl req -new -x509 -key ' + CERT_PATH + 'localhost.key -out ' + CERT_PATH + 'localhost.cert -days ' + EXPIRATION_DAYS + ' -subj /CN=localhost',
          {shell: '/bin/sh'},
          (err, stdout, stderr) => {
            if (err) {
              console.log((new Date()).toISOString() + '> X.509 certificate generation fail!\n' + err, stderr)
            } else {
              console.log((new Date()).toISOString() + '> X.509 certificate have been generated. ', stdout)
            }
          })
      }
    })
}

module.exports = {
  certGen: certGen,
  CERT_PATH: CERT_PATH
}
