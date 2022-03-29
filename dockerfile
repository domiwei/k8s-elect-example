FROM golang:1.17-alpine
# Set up apk dependencies
ENV PACKAGES make git bash linux-headers
ENV GO111MODULE=on

WORKDIR /opt/app

# Add source files
COPY . .
# Install
ENV GIT_TERMINAL_PROMPT=1
ENV GOPRIVATE=github.com/node-real
RUN go build -o elect

RUN chmod 755 ./elect

# Run the app
#CMD tail -f /dev/null
CMD ./elect
