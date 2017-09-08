```bash
docker stack deploy -c proxy.yml proxy

# Service directly

curl -H 'Host: test.local' http://localhost:8080

ab -n 5000 -c 10 "http://localhost:8080/"

for i in {1..200}
do
 # replace the ip with your manager node..
 curl -H 'Host: test.local' http://localhost:88
done


# Service through DFP

curl -H 'Host: test.local' http://localhost:88

ab -n 5000 -c 10 -H "Host: test.local" "http://localhost/"

for i in {1..200}
do
    curl -H 'Host: test.local' http://localhost:88
done
```