process.env.NODE_ENV = 'test'

const describe = require('mocha').describe
const it = require('mocha').it
const chai = require('chai')
const chaiHttp = require('chai-http')
const app = require('../app')
const server = app.listen(process.env.PORT || 3000)
const should = chai.should()

chai.use(chaiHttp)

describe('Phoenix', function () {
  it('should render home on / GET', done => {
    chai.request(server)
      .get('/')
      .end((err, res) => {
        if (err) return done(err)
        res.should.be.html
        res.should.have.status(200)
        done()
      })
  })
})
