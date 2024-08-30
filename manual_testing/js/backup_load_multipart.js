let headersList = {
    "Accept": "*/*",
    "User-Agent": "Thunder Client (https://www.thunderclient.com)",
    "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjUxMTU1MjgsImlzcyI6InRvbW8ifQ.vxD16QjP5vR_dHw45d-EE65RvhMa35VxWIitYUJNgYM"
   }
   
   let bodyContent = new FormData();
   bodyContent.append("file", "/home/tskr/docker-runner-api/src/backup.sh");
   
   let response = await fetch("localhost:7777/backup/load?file=true", { 
     method: "POST",
     body: bodyContent,
     headers: headersList
   });
   
   let data = await response.text();
   console.log(data);
   