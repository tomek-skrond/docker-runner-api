async function main() {
  let headersList = {
    "Accept": "*/*",
    "User-Agent": "Thunder Client (https://www.thunderclient.com)",
    "Content-Type": "application/json"
  };
   
  let bodyContent = JSON.stringify({
    "username" : "tomo",
    "password" : "12341234"
  });

  let response = await fetch("http://localhost:7777/login", { 
    method: "POST",
    body: bodyContent,
    headers: headersList
  });

  let data = await response.text();
  console.log(data);
}

// Call the async function
main();
