var host = "/api";
var host1 = "/uniPrint";
var hostlang=getLocal("hostlang")||"1";
function getLocal(key){
    if (window.localStorage) {
        return localStorage.getItem(key);
    }
}



