async function main(){
  let headersList = {
    "Accept": "*/*",
    "User-Agent": "Thunder Client (https://www.thunderclient.com)",
    "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjU0ODgwMTAsImlzcyI6InRvbW8ifQ.3keU7SsYZhuLSryOC9_ZGXGrE94Ypevz9vLkoTrarZg"
   }
   
   let response = await fetch("localhost:7777/stop", { 
     method: "POST",
     headers: headersList
   });
   
   let data = await response.text();
   console.log(data);
   
}

main();
