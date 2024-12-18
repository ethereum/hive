ARG baseimage=meldsun/samba
ARG tag=latest
FROM $baseimage:$tag
RUN echo "latest" > /version.txt

ENTRYPOINT ["samba-entry.sh"]


USER root

ADD samba.sh /opt/samba/bin/samba.sh
RUN chmod +x /opt/samba/bin/samba.sh
ENTRYPOINT ["/opt/samba/bin/samba.sh"]

