var Common = {
    //验证qq号码
    regQQ: function (str) {
        var reg = /^\s*\d{4,12}\s*$/g;
        return reg.test(str);
    },
    regEmail: function (str) {
        var reg = /^([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+@([a-zA-Z0-9]+[_|\_|\.]?)*[a-zA-Z0-9]+\.[a-zA-Z]{2,3}$/;
        return reg.test(str);
    },
    regTel: function (str) {
        var reg = /^[1][3,4,5,7,8][0-9]{9}$/;
        return reg.test(str);
    },
    regName: function (str) {
        var reg = /^[a-zA-Z0-9]+$/;
        return reg.test(str);
    },
    // 删除本地存储条目
    removeSession: function (key) {
        if (window.sessionStorage) {
            sessionStorage.removeItem(key);
        }
    },
    // 自动添加到本地存储
    getSession: function (key) {
        if (window.sessionStorage) {
            return sessionStorage.getItem(key);
        }
    },// 自动添加到本地存储
    addSession: function (key, data) {
        if (window.sessionStorage) {
            sessionStorage.setItem(key, data);
        }
    },
    // 删除本地存储条目
    removeLocal: function (key) {
        if (window.localStorage) {
            localStorage.removeItem(key);
        }
    },
    // 自动获取本地存储
    getLocal: function (key) {
        if (window.localStorage) {
            return localStorage.getItem(key);
        }
    },
    // 自动添加到本地存储
    addLocal: function (key, data) {
        if (window.localStorage) {
            localStorage.setItem(key, data);
        }
    },
    bindTemplate: function (id, data) {
        var html = template(id, data);
        return html
    },
    // 20100910==》2010-09-10
    fomateDate: function (str, joinstr) {
        str += '';
        var y = str.substring(0, 4);
        var m = str.substring(4, 6);
        var d = str.substring(6, 8);
        return y + joinstr + m + joinstr + d;
    },
    // 把时间700/1000---10:00格式
    fommateTimeToHHMM: function (str, joinstr) {
        var result = '';
        str += "";
        for (var i = str.length; i < 6; i++) {
            str = "0" + str;
        }
        result = str.substring(0, 2) + joinstr + str.substring(2, 4) + joinstr + str.substring(4, 6);
        return result;
    },
    // 返回20181010
    getTimeString: function (joinstr, num) {
        joinstr = joinstr || ".";
        var t = new Date();
        num = num || t;
        t.setTime(num);
        // t=num||t;
        // console.log(num)
        var y = t.getFullYear();
        var m = (t.getMonth() + 1) < 10 ? 0 + "" + (t.getMonth() + 1) : (t.getMonth() + 1);
        var d = (t.getDate()) < 10 ? 0 + "" + (t.getDate()) : (t.getDate());
        return y + joinstr + m + joinstr + d;
    },
    // 监听页面所有ajax
    // fn:所有ajax结束后要做的事
    listenAjax: function () {
        var ajaxBack = $.ajax;
        var ajaxCount = 0;
        var allAjaxDone = function () {
            $(".all-loading").fadeOut();
            $(".allwrap").show();
        } //一行代码，就可以知道所有ajax请求什么时候结束
        //由于get/post/getJSON等，最后还是调用到ajax，因此只要改ajax函数即可
        $.ajax = function (setting) {
            ajaxCount++;
            var cb = setting.complete;
            setting.complete = function () {
                if ($.isFunction(cb)) {
                    cb.apply(setting.context, arguments);
                }
                ajaxCount--;
                if (ajaxCount == 0 && $.isFunction(allAjaxDone)) {
                    allAjaxDone();
                }
            }
            ajaxBack(setting);
        }

    },
    IEVersion: function () {
        var userAgent = navigator.userAgent; //取得浏览器的userAgent字符串
        var isIE = userAgent.indexOf("compatible") > -1 && userAgent.indexOf("MSIE") > -1; //判断是否IE<11浏览器
        var isEdge = userAgent.indexOf("Edge") > -1 && !isIE; //判断是否IE的Edge浏览器
        var isIE11 = userAgent.indexOf('Trident') > -1 && userAgent.indexOf("rv:11.0") > -1;
        if (isIE) {
            var reIE = new RegExp("MSIE (\\d+\\.\\d+);");
            reIE.test(userAgent);
            var fIEVersion = parseFloat(RegExp["$1"]);
            if (fIEVersion == 7) {
                return 7;
            } else if (fIEVersion == 8) {
                return 8;
            } else if (fIEVersion == 9) {
                return 9;
            } else if (fIEVersion == 10) {
                return 10;
            } else {
                return 6;//IE版本<=7
            }
        } else if (isEdge) {
            return 'edge';//edge
        } else if (isIE11) {
            return 11; //IE11
        } else {
            return -1;//不是ie浏览器
        }
    },
    // 拦截jquery的ajax
    beforeajax: function () {
        $.ajaxSetup({
            beforeSend: function (jqXHR, settings) {
                settings.url = "" + settings.url;
            }
        });
    },
    convertURL: function (url) {
        //获取时间戳
        var timstamp = (new Date()).valueOf();
        //将时间戳信息拼接到url上
        //url = "AJAXServer"
        if (url.indexOf("?") >= 0) {
            url = url + "&t=" + timstamp;
        } else {
            url = url + "?t=" + timstamp;
        }
        return url;
    },
    //对象数组排序
    //key:比较的标准；
    //tararr:等待排序的数组
    // flag:1从大到小；0从小到大
    sort: function (tararr, key, flag) {
        for (var j = 0; j < tararr.length - 1; j++) {
            //两两比较，如果前一个比后一个大，则交换位置。
            for (var i = 0; i < tararr.length - 1 - j; i++) {
                if ((flag == 0) || (flag == "asc")) {
                    if (Number(tararr[i][key]) > Number(tararr[i + 1][key])) {
                        var temp = tararr[i];
                        tararr[i] = tararr[i + 1];
                        tararr[i + 1] = temp;
                    }
                } else {
                    if (Number(tararr[i][key]) < Number(tararr[i + 1][key])) {
                        var temp = tararr[i];
                        tararr[i] = tararr[i + 1];
                        tararr[i + 1] = temp;
                    }
                }
            }
        }
        return tararr;
    },
    // 把数组从后往前排
    turnArr: function (data) {
        var result = [];
        for (var i = data.length - 1; i >= 0; i--) {
            result.push(data[i]);
        }
        return result;
    },
    // 获得显示的书名
    checkChar: function (str) {
        var index = 0;
        var targetarr = [];
        var arr1 = str.split(".");
        var str = arr1[0];
        for (var i = 0; i < str.length; i++) {
            if (str.charCodeAt(i) > 255) //如果是汉字
            {
                index++;
            }
            else {
                index += 0.5;
            }
            targetarr.push(str.charAt(i));
            if (index > 7) {
                return targetarr.join("") + "..." + arr1[1];
            }
        }
        if (index <= 7) {
            return str + "." + arr1[1];
        }
    },
    //获取地址栏参数
    getURLQuery: function (name) {
        var reg = new RegExp("(^|&)" + name + "=([^&]*)(&|$)");
        var query = window.location.search.substr(1).match(reg);
        if (query != null) return decodeURI(query[2]);
        return null;
    },
    select: function (id, $target) {
        $target.val($("#" + id).find("li.active").attr("data-value"));
        $("#" + id).on("click", ".clickbox", function () {
            $(this).parent().siblings("ul").show();
            $("body").on("click", (function (id) {
                return function (e) {
                    if (($("#" + id).find(".clickbox").get(0) != e.target)) {
                        $("#" + id).find("ul").hide();
                    }
                }
            })(id));
        });
        $("#" + id).on("click", "li", function () {
            var $this = $(this);
            $this.addClass("active").siblings().removeClass("active");
            $this.parent().siblings(".info").find(".select").html($this.html());
            $this.parent().hide();
            $target.val($this.attr("data-value"));
        });
    },
    getCookie: function (c_name) {
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
    },
    //书名超出长度加上...
    cut: function (str, len) {
        var str_length = 0;
        var str_len = 0;
        var str_cut = new String();
        str_len = str.length;
        for (var i = 0; i < str_len; i++) {
            a = str.charAt(i);
            str_length++;
            if (a.length > 4) {
                //中文字符的长度经编码之后大于4
                str_length++;
            }
            str_cut = str_cut.concat(a);
            if (str_length >= len) {
                str_cut = str_cut.concat("...");
                return str_cut;
            }
        }
        //如果给定字符串小于指定长度，则返回源字符串；
        if (str_length < len) {
            return str;
        }
    },
    //第三方登录
    thirdlogin: function (backurl) {
        $.ajax({
            url: host + '/client/Auth/SSoPage' + "?backurl=" + backurl,
            type: 'get',
            success: function (redata) {
                let set_xinanzhengfa = Common.getCookie("set_xinanzhengfa")
                if (redata.result && set_xinanzhengfa != 1) {
                    if (backurl.indexOf("help") == -1) {
                        Cprint.createShowMes("未登录");
                        Common.addSession("backurl", backurl);
                        Common.addSession("loginState", "true");
                        window.location.href = redata.result;
                    }
                } else {
                    if (backurl.indexOf("help") == -1) {
                        Cprint.createShowMes("未登录");
                        Common.addSession("backurl", backurl);
                        window.location.href = "./login.html";
                    }
                }
            }
        });
    }
};


var Cprint = {
    // 登录类型
    judgeLoginType: function () {
        var logintype = Common.getCookie("setlogin");
        if ((logintype.indexOf("qr") != -1) && (logintype.indexOf("password") != -1)) {
        } else if (logintype.indexOf("qr") != -1) {
            $("#login").addClass("style2").removeClass("style1");
        } else if (logintype.indexOf("password") != -1) {
            $("#login").addClass("style1").removeClass("style2");
        }
        $("#login").show();
    },
    // 是否显示修改密码
    judgeChangePsdShow: function () {
        var set_change_password = Common.getCookie("set_change_password");
        if (set_change_password == 0) {
            $("#changepwbtn_js").remove();
        }
    },
    // 是否显示中英文切换
    judgeChangeLangShow: function () {
        var set_show_langbtn = Common.getCookie("set_show_langbtn");
        if (set_show_langbtn == 0) {
            $("#langwrap_js").remove();
        }
    },
    // 判断色彩
    judgePrintColor: function () {
        var set_print_color = Common.getCookie("set_print_color");
        if ((set_print_color.indexOf("black") != -1) && (set_print_color.indexOf("colorful") != -1)) {
        } else if (set_print_color.indexOf("black") != -1) {
            $("#colorful_wrap_js").remove();
        } else if (set_print_color.indexOf("colorful") != -1) {
            $("#black_wrap_js").remove();
        }
    },
    //判断纸型
    judgePrintType: function () {
        var set_print_dwPaperId = Common.getCookie("set_print_dwPaperId");
        if(set_print_dwPaperId.indexOf("不指定") == -1){
            $(".firstType").remove();
        }
        if(set_print_dwPaperId.indexOf("A4") == -1){
            $(".secondType").remove();
        }
        if(set_print_dwPaperId.indexOf("A3") == -1){
            $(".thirdType").remove();
        }
    },
    // 传入文件名称，获取对应文件归类
    getDocType: function (name) {
        var re_arr = name.split(".");
        var ilen = re_arr.length;
        var type = re_arr[ilen - 1].toLocaleLowerCase();
        if (type.indexOf("[") != -1) {
            var typearr = type.split("[");
            type = typearr[0];
        }
        var result = {
            type: Common.cut(type, 5),
            classtype: 0
        };
        if ((type == "jpg") || (type == "png") || (type == "jpeg") || (type == "png") || (type == "dif")) {
            result.classtype = "img";
        } else if (type == "pdf") {
            result.classtype = "pdf";
        } else if ((type == "txt") || (type == "xml") || (type == "xml") || (type == "mht") || (type == "mhtml") || (type == "html") || (type == "htm")) {
            result.classtype = "txt";
        } else if ((type == "doc") || (type == "docx") || (type == "dotx") || (type == "docm") || (type == "wps") || (type == "wpt") || (type == "dot") || (type == "dotm") || (type == "odt") || (type == "csv")) {
            result.classtype = "word";
        } else if ((type == "xls") || (type == "xslx") || (type == "xlsx") || (type == "xlsm") || (type == "xlsb") || (type == "xls") || (type == "ods") || (type == "dotm")) {
            result.classtype = "excel";
        } else if ((type == "ppt") || (type == "pptx") || (type == "pps") || (type == "pptm") || (type == "ppsm") || (type == "potm") || (type == "pot")) {
            result.classtype = "ppt";
        } else {
            result.classtype = "other"
        }
        return result;

    },
    createHeader: function (userInfo) {
        var set_xinanzhengfa = Common.getCookie("set_xinanzhengfa");
        var vPhoneURL = "";
        var vQRURLxnzf = "";
        var loginState = Common.getSession("loginState");
        if (set_xinanzhengfa == 1 && !loginState) {
            $.ajax({
                url: host + '/client/Auth/SSOQRPagexnzf' + "?type=1",
                type: 'get',
                async: false,
                success: function (redata) {
                    if (redata.result) {
                        vPhoneURL = redata.result;
                    }
                }
            });
            $.ajax({
                url: host + '/client/Auth/SSOQRPagexnzf' + "?type=2",
                type: 'get',
                async: false,
                success: function (redata) {
                    if (redata.result) {
                        vQRURLxnzf = redata.result;
                    }
                }
            });
        }
        var str = "";
        str += '<header><div class="container clearfix">';
        str += '    <h1 class="fl fs22"><img src="./image/logo1.png" alt=""></h1>';
        str += '    <div class="right fr clearfix fs16 ">';
        str += '        <div id="langwrap_js" class="langwrap fl relative" style="margin-right:200px">';
        str += '            <a class="langbtn_zn langbtn active" data-type="zn" href="javascript:;">中文</a>';
        str += '<span style="color:#fff;"> /</span>';
        str += '            <a class="langbtn_en first langbtn" data-type="en" href="javascript:;">English</a>';
        str += '            <span class="line"></span>';
        str += '        </div>';

        str += '      <div class="loginwrap_js loginwrap fl relative">';
        str += '           <a class="nologin relative"><i class="icon_mine"></i><span class="idQuery" data-id="640108">请登录</span></a>';
        str += '       </div>';
        str += '   </div>';
        str += '</div></header>';
        $(".wrap").prepend(str);

        var str2 = "";
        str2 += '      <div id="truelogin" class=" relative"> <div class="container login"><div id="" class="loginwrap_js loginwrap fl relative">';
        str2 += '           <a class="nologin relative"><i class="icon_mine"></i><span class="idQuery" data-id="640108">请登录</span></a>';
        str2 += '       </div></div></div>';

        $(".wrap").prepend(str2);

        var str3 = "";
        console.log(vPhoneURL)
        if (vPhoneURL != "") {
            str3 += '      <div class="otherlogin fl">';
            str3 += '           <a  href=' + vPhoneURL + ' class="relative"><img src="./images/phoneico.png"/><span class="idQuery">手机登录</span></a>';
            str3 += '       </div>';
            $(".wrap .login").prepend(str3);
        }
        var str4 = "";
        if (vQRURLxnzf != "") {
            str4 += '      <div class="otherlogin fl" style="right:16%">';
            str4 += '           <a href=' + vQRURLxnzf + ' class="relative"><img src="./images/wxico.png"/><span class="idQuery">微信登录</span></a>';
            str4 += '       </div>';
            $(".wrap .login").prepend(str4);
        }

        // if (vPhoneURL != "") {
        //     str3 += '      <div id="truelogin" class=" relative"> <div class="container"><div id="" class="loginwrap_js loginwrap fl relative">';
        //     str3 += '           <a href=' + vPhoneURL + ' class=" relative"><i class="icon_mine"></i><span class="idQuery" data-id="640108">请登录</span></a>';
        //     str3 += '       </div></div></div>';

        // }



        /*
        if(vQRURLxnzf!="")
        {
            str2 += '      <div id="truelogin" class=" relative"> <div class="container"><div id="" class="loginwrap_js loginwrap fl relative">';
            str2 += '           <a href='+vQRURLxnzf+' class="relative"><i class="icon_mine"></i><span class="idQuery" data-id="640108">请登录</span></a>';
            str2 += '       </div></div></div>';
        
        }*/



        $(".nologin").on('click', function () {
            $.ajax({
                url: host + '/client/Auth/SSoPage' + "?backurl=" + window.location.href,
                type: 'get',
                success: function (redata) {
                    var set_xinanzhengfa = Common.getCookie("set_xinanzhengfa");
                    if (redata.result && set_xinanzhengfa != 1) {
                        window.location.href = redata.result;
                    } else {
                        window.location.href = "./login.html";
                    }
                }
            });
        })

        Cprint.judgeChangeLangShow();
        // 中英文切换
        $(".langbtn").on("click", function () {
            var flag = $(this).attr("data-type");
            if (flag == "en") {
                hostlang = 2;
                Common.addLocal("hostlang", 2);
            } else {
                hostlang = 1;
                Common.addLocal("hostlang", 1);
            }
            Cprint.switchLang();
            // window.location.reload();
        });
    },
    bindPersonHtml: function (szTrueName) {
        var str = "";
        // str+='sdsdsd';
        str += ' <div id="loginselect" class="login relative sex0" href="javascript:;"><i class="icon_sex"></i><span class="sz_username">' + szTrueName + '</span>';
        str += '<div class="userinfo clearfix">';
        str += '<i class="icon_tri"></i>';
        str += '    <ul>';
        str += '        <li class="first"><a class="idQuery" data-id="640098" href="./mine.html">个人信息</a></li>';
        str += '        <li class="last"><a class="unbind-btn idQuery"  data-id="640099" href="javascript:;">退出</a></li>';
        str += '    </ul>';
        str += '</div></div>';
        $(".loginwrap_js").html(str);
        // 退出登录
        $(".loginwrap_js ").on("click", ".unbind-btn", function () {
            // 退出登录
            Cprint.otherOut(window.location.href);

        });
    },
    // 退出登录
    otherOut:function(href) {
        $.ajax({
            url: host + '/client/Auth/Logout?timestamp=' + Math.random(),
            type: 'get',
            success: function (redata) {
            if (redata.code == 0) {
                        Common.removeSession("loginState");
                        Common.removeSession("userInfo");
						if (redata.result)
							window.location.href = redata.result + "?redirectUrl=" + href;
						else
							window.location.href = "./help.html";
                    } else {
                        if (hostlang == 2) {
                            Cprint.createShowMes("Exit failure");
                        } else {
                            Cprint.createShowMes("退出失败");
                        }
                    }
                },
                complete: function (e) {
                    Cprint.complete(e);
                },
                error: function (redata) {
                    Cprint.errorAjax();
                }
            });
    },
    createBanner: function () {
        var str = "";
        str += '<div class="banner">';
        str += '<img class="bgimg" src="./image/banner.png">';
        str += '<div class="container"><div class="text"><h4 class="idQuery" data-id="610023">杭州打印管理系统</h4> ';
        str += '<p class="idQuery" data-id="640115">颠覆传统打印复印模式，更方便、更经济、更省时 </p> ';
        str += ' </div><div class="imgbox"><img src="./image/banner-tip.png" alt=""></div>';
        str += '</div></div>';
        $(".wrap").prepend(str);
    },
    createNav: function () {
        var isDisabledscanbtn = Common.getCookie("set_isDisabledscanbtn");//获取配置项
        var str = "";
        str += '<nav>';
        str += '    <ul class="container clearfix">';
        str += '        <li class=" first"><a class=" nav_c1" data-id="640001" href="./cprint.html"><div class="imgbox"><img src="./image/c1.png"></div><h6 class="idQuery nav_c1" data-id="640001">云打印</h6></a></li>';
        str += '        <li class=""><a class=" nav_c2" data-id="640073" href="./printDoc.html"><div class="imgbox"><img src="./image/c2.png"></div><h6 class="idQuery nav_c2" data-id="640073">打印文档</h6></a></li>';
        if(isDisabledscanbtn == "false"){
            str += '    <li class=""><a class=" nav_c3" data-id="640002" href="./scanDoc.html"><div class="imgbox"><img src="./image/c3.png"></div><h6 class="idQuery nav_c3" data-id="640002">扫描文档</h6></a></li>';
		}
        str += '        <li class=""><a class=" nav_c4" data-id="640071" href="./printDev.html"><div class="imgbox"><img src="./image/c4.png"></div><h6  class="idQuery nav_c4" data-id="640071">打印点</h6></a></li>';
        str += '        <li class=""><a class=" nav_c5" data-id="640006" href="./footprint.html"><div class="imgbox"><img src="./image/c5.png"></div><h6 class="idQuery nav_c5" data-id="640006" >使用记录</h6></a></li>';
        str += '        <li class=" last"><a class=" nav_c6" data-id="640050" href="./help.html"><div class="imgbox"><img src="./image/c6.png"></div><h6 class="idQuery nav_c6" data-id="640050">帮助</h6></a></li>';
        str += '    </ul>';
        str += '</nav>';
        $(".wrap").prepend(str);
        if(isDisabledscanbtn != "false"){
            $("nav li").addClass("navli");
        }else{
            $("nav li").removeClass("navli");
        }
       
        $("nav li a").click(function () {
            // $(this).addClass("active").siblings().removeClass("active");
            gonav($(this).attr("class"));
        });
        $("nav .imgbox").click(function () {
            gonav($(this).parent().attr("class"));
        });
        $("nav img").click(function () {
            gonav($(this).parent().parent().attr("class"));
        });
        $("nav h6").click(function () {
            gonav($(this).parent().attr("class"));
        });

        function gonav(gourltype) {
            if (gourltype.indexOf("nav_c1") != (-1)) {
                window.location.href = "./cprint.html";
            } else if (gourltype.indexOf("nav_c2") != (-1)) {
                window.location.href = "./printDoc.html";
            } else if (gourltype.indexOf("nav_c3") != (-1)) {
                window.location.href = "./scanDoc.html";
            } else if (gourltype.indexOf("nav_c4") != (-1)) {
                window.location.href = "./printDev.html";
            } else if (gourltype.indexOf("nav_c5") != (-1)) {
                window.location.href = "./footprint.html";
            } else if (gourltype.indexOf("nav_c6") != (-1)) {
                window.location.href = "./help.html";
            }
        }
    },
    createfooter: function () {
        var time1 = "2010-";
        var d = new Date();
        time1 += d.getFullYear();

        $(".footer").html("<div class='footerwrap'><p>©" + time1 + "<span class='idQuery' data-id='640117'>版权所有   杭州联创信息技术有限公司</span>    "+Common.getCookie("set_version")+"</p></div>");

    },
    errorAjax: function () {
        // console.log("请求失败");
    },
    returnAjax: function (data) {
        if (data.code == 4) {
            if (window.location.href.indexOf("errorCode") != -1) {
                let errorCode = Common.getURLQuery("errorCode");
                let href = window.location.href.split("?")[0];
                if (errorCode == 16777228 || errorCode == 0x100000C) {
                    let msg = Common.getCookie("set_errormsg");
                    Cprint.createShowMes(msg);
                    setTimeout(() => {
                        Cprint.otherOut(href);
                    }, 10000)
                } else {
                    Cprint.createShowMes("其它错误，请重新登陆")
                    setTimeout(() => {
                        Cprint.otherOut(href);
                    }, 10000)
                }
            } else {
                Common.thirdlogin(window.location.href)
            }
        } else if (data.code != 0) {
            Cprint.createShowMes(data.message || "后台出错");
            return false;
        } else {
            return true;
        }
    },
    // 验证是否登录
    check: function (fn) {
        Cprint.switchLang();
        $.ajax({
            url: host + '/client/Auth/Check',
            type: 'post',
            // dataType: 'JSON',
            // contentType: 'application/json',
            success: function (redata) {
                if ((redata.code != 0)) {
                    if (window.location.href.indexOf("errorCode") != -1) {
                        let errorCode = Common.getURLQuery("errorCode");
                        let href = window.location.href.split("?")[0];
                        if (errorCode == 16777228 || errorCode == 0x100000C) {
                            let msg = Common.getCookie("set_errormsg");
                            Cprint.createShowMes(msg);
                            setTimeout(() => {
                                Cprint.otherOut(href);
                            }, 3000)
                        } else {
                            Cprint.createShowMes("其它错误，请重新登陆")
                            setTimeout(() => {
                                Cprint.otherOut(href);
                            }, 3000)
                        }
                    } else {
                        Common.thirdlogin(window.location.href)
                    }
                } else {
                    $(".all-loading1").fadeOut();
                    Common.addSession("SessionId", redata.SessionId);
                    $(".username").html(redata.result.szTrueName);
                    $("#tel").val(redata.result.szHandPhone);
                    $("#email").val(redata.result.szEmail);
                    $("#username").val(redata.result.szLogonName);
                    Cprint.bindPersonHtml(redata.result.szTrueName);
                    $("#szname_x").html(redata.result.szLogonName);
                }
                Cprint.switchLang();
            },
            error: function (redata) {
                Cprint.errorAjax();
            }
        });
    },
    // 确认删除框
    createDelModel: function () {
        var str = "";
        str += '<div class="modelwrap">';
        str += '    <div class="maskbg"></div>';
        str += '    <div class="modelbody fs14">';
        str += '        <div class="text idQuery" data-id="640034">确定删除此文档？</div>';
        str += '        <div class="btnbox clearfix">';
        str += '            <a class="close-btn idQuery" data-id="640035" href="javascript:;">取消</a>';
        str += '            <a class="confirm-btn close-btn idQuery" data-id="640033" href="javascript:;">删除</a>';
        str += '        </div>';
        str += '       <a href="javascript:;" class="outbtn close-btn">';
        str += '           <i class="icon_close"></i>';
        str += '       </a>';
        str += '    </div>';
        str += '</div>';
        $("body").append(str);
        var $modelwrap = $(".modelwrap");
        $modelwrap.find(".close-btn").click(function () {
            $modelwrap.hide();
            $(".wrap").removeClass("over_h");
        });
    },
    // 接口返回信息
    createShowMes: function (info) {
        var str = "";
        str += '<div class="mes">';
        str += '    <div class="maskbg"></div>';
        str += '    <div class="mesbody fs14">';
        str += '    <p>' + info + '</p>';
        str += '    <i class="icon_mes"></i>';
        str += '    <a class="close-btn" href="javascript:;"><i class="icon_close"></i></a>';
        str += '</div>';
        str += '</div>';
        $("body").append(str);
        var $mesbox = $(".mes");
        $("body").on("click", ".close-btn", function () {
            $mesbox.hide().remove();
        });
    },
    // 接口返回信息
    createSuc: function (info, fn) {
        var str = "";
        str += '<div class="tipbox hide">';
        str += '    <div class="tipbody">' + info + '</div>';
        str += '</div>';
        $("body").append(str);
        var $mesbox = $(".tipbox");
        $mesbox.fadeIn("fast")
            .delay(3000).fadeOut("normal", function () {
                $mesbox.remove();
                if (typeof fn == "function") {
                    fn();
                }
            });
    },
    complete: function (e) {
        $(".loading").hide();
        if (e.status == 504) {
            Cprint.createShowMes("网络故障");
        }
    },
    switchLang: function () {
        // return;
        // var lang = Number(Common.getLocal("lang"));
        if (hostlang == 2) {
            // $(".lang-wrap").html("English");
            $(".langbtn_en").addClass("active").siblings(".langbtn").removeClass("active");
            $("input.ch-calender").hide();
            $("input.en-calender").show();
        } else {
            // $(".lang-wrap").html("中文");
            $(".langbtn_zn").addClass("active").siblings(".langbtn").removeClass("active");
            $("input.ch-calender").show();
            $("input.en-calender").hide();
        }
        var langinfo = Common.getSession("langinfo");
        if (langinfo) {
            bindTextLang(JSON.parse(langinfo));
            return;
        }
        // 获取
        $.ajax({
            url: host + '/system/MulLan/Get?dwSubSysSN=6&timestamp=' + new Date().getTime(),
            type: 'get',
            success: function (redata) {
                if (redata.code != 0) {
                    return;
                }
                Common.addSession("langinfo", JSON.stringify(redata.result));
                bindTextLang(redata.result);
            },
            error: function (redata) {
                Cprint.errorAjax();
            }
        });

        function bindTextLang(data) {
            if (data != "") {
                if (hostlang == 2) {
                    for (var i = 0; i < data.length; i++) {
                        var item = data[i];
                        if (item.dwLanSN == 2) {
                            var dwId = item.dwTextID;
                            var vObj = $("body").find(".idQuery[data-id='" + dwId + "']");
                            vObj.html(item.szTextInfo);


                        };
                    }
                } else {
                    for (var i = 0; i < data.length; i++) {
                        var item = data[i];
                        if (item.dwLanSN == 1) {
                            var dwId = item.dwTextID;
                            //$("body").find(".idQuery[data-id='" + dwId + "']").html(item.szTextInfo);
                            var vObj = $("body").find(".idQuery[data-id='" + dwId + "']");
                            vObj.html(item.szTextInfo);
                        }
                    }
                }
            }
        }
    },
    getPaperType: function (val) {
        var paperType = Common.getSession("paper-type");
        if (paperType) {
            return backPaperName(val, JSON.parse(paperType));
        }
        // 1.获取纸张类型
        $.ajax({
            url: host + '/client/Paper/GetPaper',
            type: 'post',
            // dataType: 'JSON',
            // contentType: 'application/json',
            success: function (redata) {
                if (redata.code != 0) {
                    return;
                }
                Common.addSession("paper-type", JSON.stringify(redata.result));
                backPaperName(val, redara.result);
            },
            error: function (redata) {
                Cprint.errorAjax();
            }
        });

        function backPaperName(val, data) {
            var name = "";
            for (var i = 0; i < data.length; i++) {
                if (data[i].dwPaperID == val) {
                    name = data[i].szPaperName;
                }
            }
            return name;
        }
    },
    fn: function () {
        var papertype = Common.getSession("paper-type");
        if (!papertype) {
            // 1.获取纸张类型
            $.ajax({
                url: host + '/client/Paper/GetPaper',
                type: 'post',
                // dataType: 'JSON',
                // contentType: 'application/json',
                success: function (redata) {
                    if (redata.code != 0) {
                        return;
                    }
                    Common.addSession("paper-type", JSON.stringify(redata));
                },
                error: function (redata) {
                    Cprint.errorAjax();
                }
            });
        }
    }
};

function addCommon() {
    Cprint.createNav();
    Cprint.createBanner();
    Cprint.createHeader();
    Cprint.createfooter();
}