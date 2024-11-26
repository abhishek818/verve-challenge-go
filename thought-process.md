1. ./main.go - the parent folder file contains logic for the initial requirements.

2.  ./extensions/main.go - all requirements stated in 'Extension' section are implemented here.

Logic behind ./main.go file:

    - Have used the built-in 'net/http' package for http requests. It handles multiple requests 
      concurrently using goroutines.
    - Validated mandatory and optional query parameters.
    - Response handling for both success and failed scenarios.
    
    Logging :
    - Used a sync.Map for storing unique ids. (alternative is a map with read-write mutex)
    - Used goroutine and 'time.Ticker' to trigger the logging action every minute and delete 
      the old keys from map.
    - 'log' package for writing logs to a file.

    'endpoint' param:
    - Lookup the same map for count of unique ids.


Logic behind ./extensions/main.go file:

    - Ext 1: Post request for 'endpoint' param url with count and timestamp fields.
    
    - Ext 2: For id deduplication to work in a distributed environment (load balancer based 
      deployments), we need to store the id in a distributed cache like Redis. Lookups and insertion of unique ids (even from different server instances) are handled this way.
      
      Always delete the old keys from the cache every minute which acts as a TTL for keys to avoid indefintie storage and repetitive counts. 
    
    - Ext 3: Send the count of unique received ids to a distributed streaming service like 
      Kafka.
    - Create a topic called 'unique-id-count' and Publish counts of unique ids to the same.


Docker Setup:

    - For Redis and Kafka setup, respective docker containers are used.
    - While connecting to the kafka instance during creation of topic, Wait and keep several 
      retries for the kafka to get properly initialized first.

