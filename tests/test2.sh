# requires curl, kubectl and redis-cli
# TODO: add checks for side effects (check ressource statuses on k8s and values on redis)

response_code=$(curl --write-out "%{http_code}\n" \
    --silent \
    --output init_session.txt \
    --location \
    --request POST 'http://localhost:8080/init/hamza-test-2')

if [[ $(($response_code/100)) == 2 ]] ;
then
    echo "init session successfully"
    cat init_session.txt
else
    echo "failed to init session"
    cat init_session.txt
    exit 1
fi

response_code=$(curl --write-out "%{http_code}\n" \
    --silent \
    --output init_session.txt \
    --location \
    --request POST 'http://localhost:8080/init/hamza-test-2')

if [[ $(($response_code/100)) == 5 ]] ;
then
    echo "reinit session successfully"
    cat init_session.txt
else
    echo "reinit session failed"
    cat init_session.txt
    exit 1
fi
