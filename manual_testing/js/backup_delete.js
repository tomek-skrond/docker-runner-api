async function main() {
  let headersList = {
    "Accept": "*/*",
    "User-Agent": "Thunder Client (https://www.thunderclient.com)",
    "Authorization": "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3MjUwNTQ0NTUsImlzcyI6InRvbW8ifQ.2xZ-p_OSwCqDekdTlhcy3ShWcSYDk3XA63LKM-gvnCI"
   }
   
   let response = await fetch("localhost:7777/backup/delete?delete=finger_20240827_003427.zip", { 
     method: "DELETE",
     headers: headersList
   });
   
   let data = await response.text();
   console.log(data);
   
}

main();