# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
# Stage 1
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

FROM golang:1.22.2 as builder

ENV GO111MODULE=on

WORKDIR /app

COPY . .

RUN make local
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
# Stage 2 
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

FROM golang:1.22.2

COPY --from=builder /app/bin/ /app/
COPY views/ /app/views/
COPY static/ /app/static/
COPY auth.json /app/auth.json

WORKDIR /app

ENTRYPOINT ["/app/go-mp4-server"]