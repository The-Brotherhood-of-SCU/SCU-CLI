var set_version = "V3.0.20220822";


var setlogin = "password";//qr:扫码；password：账号密码；qr，password：两种方式
var set_change_password = 1;//修改密码1:有；0：没有
var set_xinanzhengfa = 0;//西南政法大学修改方式1:支持微信、手机号登录;其它值：不支持微信，手机号登录
var set_show_langbtn = 1;//0：不可以选择中英文；1：可以设置中英文
var set_print_color = "black,colorful";//black,colorful：黑白和彩色都有；black:只有黑白；colorful：只有彩色；
var set_print_dwPaperId = "A3,A4,不指定";//打印上传纸型配置项：A3,A4,不指定：A3、A4、不指定都有；A3:只有A3；A4：只有A4，不指定：只有不指定；
var set_uniprint_changpassword_api=1;//0:没有设置uniprint;1:设置了Uniprint：++++++这个配置项的作用是可以统一密码
var set_mobile_index_school_link = "http://zjut.cprint.unifound.net";//移动端首页pc端网址

var set_filesize = "";//上传文件的大小限制
var set_payurl = "http://selfcopying.cnbksy.com/payapi/webAuth";//云打印跳转到多钱包的地址

var set_errormsg = "云打印系统内没有您的账号信息，请联系图书馆技术部处理，电话XXXXXXXX"

var set_isDisabledscanbtn = false; //禁用网站扫描功能，true为禁用；false为不禁用
var set_mobileindexShowFlag = false;//移动端首页不同排版按钮展示，true为上图同版6个按钮的展示风格,false为原版本4个按钮加两个小图标的展示风格









setCookie("set_version", set_version, 1000000);
setCookie("setlogin", setlogin, 1000000);
setCookie("set_errormsg", set_errormsg, 1000000);
setCookie("set_change_password", set_change_password, 1000000);
setCookie("set_xinanzhengfa", set_xinanzhengfa, 1000000);
setCookie("set_show_langbtn", set_show_langbtn, 1000000);
setCookie("set_print_color", set_print_color, 1000000);
setCookie("set_print_dwPaperId", set_print_dwPaperId, 1000000);
setCookie("set_uniprint_changpassword_api", set_uniprint_changpassword_api, 1000000);
setCookie("set_mobile_index_school_link", set_mobile_index_school_link, 1000000);
setCookie("set_isDisabledscanbtn", set_isDisabledscanbtn, 1000000);
setCookie("set_mobileindexShowFlag", set_mobileindexShowFlag, 1000000);

setCookie("set_filesize", set_filesize, 1000000);

function setCookie(c_name, value, expiredays) {
    var exdate = new Date();
    exdate.setDate(exdate.getDate() + expiredays);
    document.cookie = c_name + "=" + escape(value) +
        ((expiredays == null) ? "" : ";expires=" + exdate.toGMTString());
}

function getCookie(c_name) {
    if (document.cookie.length > 0) {
        c_start = document.cookie.indexOf(c_name + "=");
        if (c_start != -1) {
            c_start = c_start + c_name.length + 1;
            c_end = document.cookie.indexOf(";", c_start);
            if (c_end == -1) c_end = document.cookie.length;
            return unescape(document.cookie.substring(c_start, c_end));
        }
    }
    return ""
}