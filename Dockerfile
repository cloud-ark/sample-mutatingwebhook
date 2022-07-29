FROM ubuntu:20.04
RUN apt-get update && apt-get install -y openssl 
ADD webhook-create-self-signed-ca-cert.sh /
COPY kubectl /root/
COPY mutatingwebhook.yaml /root/.
RUN cp /root/kubectl bin/. && chmod +x /root/kubectl && chmod +x bin/kubectl
ENTRYPOINT ["/webhook-create-self-signed-ca-cert.sh"]
