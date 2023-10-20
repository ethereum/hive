
hivechain generate \
    -outdir chain \
    -length 2000 \
    -lastfork shanghai \
    -tx-interval 5 \
    -fork-interval 10 \
    -outputs forkenv,genesis,chain,headblock,headfcu,headnewpayload
