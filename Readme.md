All design considerations can be found in ./thought-process.md file.

Setup:

1. cd to parent directory and run <go run ./main.go>

2. cd ./extensions

   docker-compose build

   docker-compose up

   open 'http://localhost:8080/' on browser
   
   Sample GET requests : 
   http://localhost:8080/api/verve/accept?id=1

   http://localhost:8080/api/verve/accept?id=1&&endpoint=https://www.google.com/
