# Fix - old clients does not be removed from metada.md
Currently, the database keeps metadata of clients that have not connected for a long time (manly because they changed its ids). This causes a lot of trash data in the database.
When starting up the vss with --allow-raw-db-access (or looking at the database dump file) is possible to se all the old clients that are not being removed from the metadata.

## proposal
Detect if the cause is the lack in the bussiness logic or another kind of bug (controller have mechanics to remove old clients) and fix or implement the solution.


```bash
vss &
pid=$!

sleep 2
#connect to port 5032 and read data (lines) until receive a line started with "sugestednewid"
exec 3<>/dev/tcp/localhost/5032
while read -u 3 line; do
    echo "Received: $line"
    if [[ $line == sugestednewid* ]]; then
        #the id is separated from the command by a ';' (sugestednewid;the id here)
        id=$(echo $line | cut -d';' -f2)
        break
    fi
done

#send a observation package to the server with the id received (subscribe:n0.tests.listToThisVar)
echo -e "subscribe;n0.tests.listToThisVar\n" >&3

sleep 1

kill $pid
```