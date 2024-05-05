htmx = require("htmx.org");

let formReceived = true;
function clearTextInput() {
  document.getElementById("text-input").value = "";
}
htmx.on("htmx:afterSwap", (event) => {
  const requestConfig = event.detail.requestConfig;
  if (requestConfig.elt.id == "form") {
    clearTextInput();
    formReceived = false;
  }
});
htmx.on("htmx:beforeRequest", (event) => {
  const requestConfig = event.detail.requestConfig;
  if (requestConfig.elt.id == "form") {
    if (!formReceived) {
      event.preventDefault();
    }
  }
});
document.addEventListener("htmx:sseMessage", (event) => {
  console.log(event);
}); // not triggered
document.addEventListener("htmx:beforeProcessNode", (event) => {
  const srcElement = event?.srcElement;
  const target = event?.target;
  if (
    srcElement &&
    target &&
    srcElement.id.startsWith("answer") &&
    target.id.startsWith("answer")
  ) {
    if (!srcElement.hasAttribute("hx-sse") && !target.hasAttribute("hx-sse")) {
      formReceived = true;
    }
  }
});
/*htmx.onLoad((elt) => {
  console.log(elt);
});
document.addEventListener("htmx:sseMessage", (event) => {
  console.log(event);
});*/
// htmx.logAll();
