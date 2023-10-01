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
    --output create_components.txt \
    --location \
    --request POST 'http://localhost:8080/create/hamza-test-2' \
    --header 'Content-Type: application/json' \
    --data-raw '{
    "components": [
        {
            "componentType": "code",
            "exposeComponent": true,
            "componentID": "my-code-editor",
            "componentMetadata": {
                "Password": "thisismypassword"
            }
        },
        {
            "componentType": "redis",
            "exposeComponent": true,
            "componentID": "my-redis",
            "componentMetadata": {
                "Password": ""
            }
        },
        {
            "componentType": "mongo",
            "exposeComponent": true,
            "componentID": "my-mongo",
            "componentMetadata": {
                "Password": ""
            }
        }
    ]
}')

if [[ $(($response_code/100)) == 2 ]] ;
then
    echo "created components successfully"
    cat create_components.txt
else
    echo "failed to create components"
    cat create_components.txt
    exit 1
fi


sleep 20

response_code=$(curl --write-out "%{http_code}\n" \
    --silent \
    --output session_refresh.txt \
    --location \
    --request POST 'http://localhost:8080/refresh/hamza-test-2')

if [[ $(($response_code/100)) == 2 ]] ;
then
    echo "refreshed session successfully"
    cat session_refresh.txt
else
    echo "failed to refresh session"
    cat session_refresh.txt
    exit 1
fi


response_code=$(curl --write-out "%{http_code}\n" \
    --silent \
    --output component_statuses.txt \
    --location \
    --request GET 'http://localhost:8080/statuses/hamza-test-2')

if [[ $(($response_code/100)) == 2 ]] ;
then
    echo "fetched component statuses successfully"
    cat component_statuses.txt
else
    echo "failed to fetch component statuses"
    cat component_statuses.txt
    exit 1
fi


response_code=$(curl --write-out "%{http_code}\n" \
    --silent \
    --output delete_session.txt \
    --location \
    --request DELETE 'http://localhost:8080/hamza-test-2')

if [[ $(($response_code/100)) == 2 ]] ;
then
    echo "deleted session successfully"
    cat delete_session.txt
else
    echo "failed to delete session"
    cat delete_session.txt
    exit 1
fi

