FROM node:6-stretch

ENV TESTS_PATH=/opt/osb-checker/2.13/tests

RUN git clone https://github.com/openservicebrokerapi/osb-checker.git /opt/osb-checker

RUN rm $TESTS_PATH/test/configs/*.json \
  && npm config set package-lock false && npm install --global mocha \
  && apt-get update \
  && apt-get install apt-transport-https \
  && apt-get install -y netcat

RUN cd /opt/osb-checker/common \
  && npm install -g \
  && npm install --save guid \
  && npm install --save supertest \
  && npm install --save jsonschema \
  && npm install --save async \
  && npm install --save underscore \
  && cd $TESTS_PATH \
  && npm install -g \
  && npm install --save guid \
  && npm install --save supertest \
  && npm install --save jsonschema \
  && npm install --save async \
  && npm install --save underscore

COPY scripts/*.sh /app/
COPY config_mock.json /app/

WORKDIR /app