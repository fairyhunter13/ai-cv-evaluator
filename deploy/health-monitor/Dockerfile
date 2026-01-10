FROM alpine:3.19

RUN apk add --no-cache curl bash

COPY health-monitor.sh /health-monitor.sh
RUN chmod +x /health-monitor.sh

CMD ["/health-monitor.sh"]
