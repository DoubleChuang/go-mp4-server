# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
# Stage 1
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

FROM golang:1.17 as builder

ENV GO111MODULE=on

WORKDIR /app

COPY . .

RUN make local
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
# Stage 2 
# - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -

FROM golang:1.17

COPY --from=builder /app/bin/ /app/
COPY views/ /app/views/

WORKDIR /app

ENTRYPOINT ["/app/go-mp4-server"]