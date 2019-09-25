/* eslint-disable no-multi-spaces */
const createError = require('http-errors')
const express     = require('express')
const path        = require('path')
const cookieParser = require('cookie-parser')
const logger      = require('morgan')
const dblogger    = require('mongo-morgan')
const mongoose    = require('mongoose')
const indexRouter = require('./routes/index')
const app = express()

// ---------- Prometheus metrics
exports.AggregatorRegistry = require('./lib/cluster')
// -----------------------------------------

// view engine setup
app.set('views', path.join(__dirname, 'views'))
app.set('view engine', 'hbs')

// -----------------------------------------
// manage db connection
console.log((new Date()).toISOString() + '> Mongo DB URI: ' + process.env.DB_CONNECTION_STRING)

const options = {
  autoIndex: false,     // Don't build indexes
  reconnectTries: 30,   // Retry up to 30 times
  reconnectInterval: 500, // Reconnect every 500ms
  poolSize: 10,         // Maintain up to 10 socket connections
  // If not connected, return errors immediately rather than waiting for reconnect
  bufferMaxEntries: 0,
  useNewUrlParser: true
}

const connectWithRetry = () => {
  console.log((new Date()).toISOString() + '> [' + process.env.DB_CONNECTION_STRING + '] MongoDB connection with retry')

  mongoose.connect(process.env.DB_CONNECTION_STRING, options).then(() => {
    console.log((new Date()).toISOString() + '> MongoDB is connected')

    app.use(dblogger(process.env.DB_CONNECTION_STRING, 'combined', {
      collection: 'logs'
    }))
  }).catch(err => {
    console.error((new Date()).toISOString() + '> MongoDB connection unsuccessful, retry after 10 seconds.\n ERROR: \n', err)
    setTimeout(connectWithRetry, 10000)
  })
}
connectWithRetry()
// -----------------------------------------

app.use(logger('combined'))
app.use(express.json())
app.use(express.urlencoded({extended: false}))
app.use(cookieParser())
app.use(express.static(path.join(__dirname, 'public')))

app.use('/', indexRouter)

// catch 404 and forward to error handler
app.use(function (_req, _res, next) {
  next(createError(404))
})

// error handler
app.use(function (err, req, res, _next) {
  // set locals, only providing error in development
  res.locals.message = err.message
  res.locals.error = req.app.get('env') === 'development' ? err : {}

  // render the error page
  res.status(err.status || 500)
  res.render('error')
})

module.exports = app
