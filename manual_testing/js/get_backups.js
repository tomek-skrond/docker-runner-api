let headersList = {
    "Accept": "*/*",
    "User-Agent": "Thunder Client (https://www.thunderclient.com)",
    "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjUwOTY1MDIsImlzcyI6InRvbW8ifQ.mkytIYCSnaYMk2kQr8zZaVfi2S-bR-jmjed3IAIaLb0"
   }
   
   let response = await fetch("localhost:7777/backup", { 
     method: "GET",
     headers: headersList
   });
   
   let data = await response.text();
   console.log(data);
   