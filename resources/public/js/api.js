
function selectAllChallenges(source) {
    let checkboxes = document.getElementsByName('challenge');
    for(let i=0, n=checkboxes.length;i<n;i++) {
        checkboxes[i].checked = source.checked;
    }
}

document.getElementById("api_request").onclick = function () {
    let challenges = []
    const checkboxes = document.querySelectorAll('input[name="challenge"]:checked');
    for (let checkbox of checkboxes) {
        challenges.push(checkbox.value);
    }
    if (challenges.length == 0) {
        const chals_field = document.getElementById('chals_field');
        chals_field.classList.add('challenges-fiels-error')
        return
    }
    window.location.href = '/api/?challenges=' + challenges.join()
};

window.onload = function(){
    connectToWS()
    addBodyClass()
}

function connectToWS() {
    let url = new URL('/challengesFrontend', window.location.href);
    url.protocol = url.protocol.replace('http', 'ws');
    let self = this;
    let ws = new WebSocket(url);
    ws.onmessage = receiveMsg;
    ws.onclose = function(){
        ws = null;
        setTimeout(function(){self.connectToWS(url)}, 3000);
    };
}

function receiveMsg(evt) {
    let frontendChallenges
    let messages = evt.data.split('\n');
    for (let i = 0; i < messages.length; i++) {
        let msg = messages[i];
        let json = JSON.parse(msg);
        if (json.msg === "challenges_categories"){
            frontendChallenges = json.values;
        }
    }
    showChallenges(frontendChallenges)
}

function showChallenges(frontendChallenges){
    const nav_pills = document.getElementById('challenges-category-nav')
    const challenges_tab = document.getElementById('challenges-tab')
    let count = 0
    for ( let i = 0; i < frontendChallenges.length; i ++ ) {
        let category = document.createElement('a');
        category.href = '#' + frontendChallenges[i].tag
        category.innerText = frontendChallenges[i].name
        category.setAttribute('data-toggle', 'pill')
        category.setAttribute('id', frontendChallenges[i].tag + '-tab')
        category.classList.add('nav-link')
        i == 0 ? category.classList.add('active') : "" ;
        nav_pills.appendChild(category)

        let cat_tab = document.createElement('div')
        cat_tab.classList.add('tab-pane','fade')
        i == 0 ? cat_tab.classList.add('active','show') : "";
        cat_tab.setAttribute('id', frontendChallenges[i].tag)
        cat_tab.setAttribute('role', 'tabpanel')
        cat_tab.setAttribute('aria-labelledby', frontendChallenges[i].tag + '-tab')
        challenges_tab.appendChild(cat_tab)

        for ( let j = 0; j < frontendChallenges[i].challenges.length; j ++ ){
            count++
            let challenge = frontendChallenges[i].challenges[j]
            let nav_tabs = document.getElementById(frontendChallenges[i].tag)
            let div = createChallengeCheckBox(count, challenge)
            nav_tabs.appendChild(div)
        }
    }
}

function createChallengeCheckBox(n, challenge){
    let div = document.createElement('div')
    div.classList.add("custom-control", "custom-checkbox")

    let input = document.createElement('input')
    input.setAttribute('type', 'checkbox')
    input.setAttribute('name', 'challenge')
    input.setAttribute('id', 'challenge' + n)
    input.setAttribute('value', challenge.tag)
    input.classList.add('custom-control-input')

    let label = document.createElement('label')
    label.setAttribute('for', 'challenge' + n)
    label.classList.add('custom-control-label')
    label.innerText = challenge.name

    div.appendChild(input)
    div.appendChild(label)

    return div
}

function addBodyClass() {
    const x = document.getElementsByTagName("BODY")[0];
    if (window.location.pathname == "/"){
        x.classList.add("home-page")
        return
    }
    x.classList.add("other-page")
}