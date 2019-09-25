#### ---------- TESTING BUILD -------------------- ####
ARG NODE_VERSION=8.11.1
FROM node:$NODE_VERSION-alpine AS build
ARG APP_PORT=3000
ARG DB_PORT=27017
ARG DB_URI=mongodb://phoenix_mongo_app:$DB_PORT/phoenix
ENV NODE_ENV=test
ENV ENV=development
ENV PORT=$APP_PORT
ENV DB_CONNECTION_STRING=$DB_URI

WORKDIR /usr/app
COPY ["*.json", "*.js", "*.ts", "./"]
COPY bin/. ./bin/
COPY lib/. ./lib/
COPY ./config/*js ./config/
COPY test/. ./test/
COPY routes/. ./routes/
COPY views/. ./views/

EXPOSE $APP_PORT
RUN npm i -g npm@latest
RUN apk add --update python make g++
RUN npm i
RUN npm audit fix
RUN npm run test
RUN npm prune --production


#### ---------- PRODUCTION IMAGE -------------------- ####
ARG NODE_VERSION=8.11.1
FROM node:$NODE_VERSION-alpine
ARG APP_PORT=3000
ARG DB_PORT=27017
ARG DB_URI=mongodb://phoenix_mongo_app:$DB_PORT/phoenix
ENV NODE_ENV=production
ENV ENV=$NODE_ENV
ENV PORT=$APP_PORT
ENV DB_CONNECTION_STRING=$DB_URI

WORKDIR /usr/app
COPY ["*.json", "*.js", "*.ts", "./"]
COPY bin/. ./bin/
COPY cert/ ./cert/
COPY lib/. ./lib/
COPY ./config/*js ./config/
COPY public/* ./public/
COPY routes/. ./routes/
COPY views/. ./views/
COPY --from=build /usr/app/node_modules node_modules
RUN apk add --update curl openssl
CMD npm start
EXPOSE $APP_PORT
