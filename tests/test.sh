# requires curl, kubectl and redis-cli
# TODO: add checks for side effects (check ressource statuses on k8s and values on redis)

curl --location --request POST 'http://localhost:8080/init/hamza-test-2'

curl --location --request POST 'http://localhost:8080/create/hamza-test-2' \
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
}'

sleep 20

curl --location --request POST 'http://localhost:8080/refresh/hamza-test-2'

curl --location --request GET 'http://localhost:8080/statuses/hamza-test-2'

curl --location --request DELETE 'http://localhost:8080/hamza-test-2'

