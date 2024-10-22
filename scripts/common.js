function includeHTML() {
  const includeElements = document.querySelectorAll('[data-include]');
  
  includeElements.forEach(el => {
    const file = el.getAttribute("data-include");
    fetch(file)
      .then(response => response.text())
      .then(data => {
        el.innerHTML = data;
        if (el.id === "header") {
          loadHeaderContent();
        }
      })
      .catch(err => console.error("Failed to load file", err));
  });
}

function loadHeaderContent() {
  const pageName = document.body.getAttribute('data-page'); // Get current page
  const menuFiles = {
    "index": "templates/menus/menu_index.html",
    "dashboard": "templates/menus/menu_dashboard.html",
	"project_details": "templates/menus/menu_project_details.html"
    // Add more cases for different pages as needed
  };

  const menuFile = menuFiles[pageName] || menuFiles['index']; // Default to index menu if no match

  // Fetch and load the appropriate menu HTML file
  fetch(menuFile)
    .then(response => response.text())
    .then(data => {
      document.getElementById('menuItems').innerHTML = data;
    })
    .catch(err => console.error("Failed to load menu file", err));
}

// Call the function on page load
window.onload = includeHTML;


function myFunction() {
	var x = document.getElementById("demo");
	if (x.className.indexOf("w3-show") == -1) {
		x.className += " w3-show";
	} else {
		x.className = x.className.replace(" w3-show", "");
	}
}