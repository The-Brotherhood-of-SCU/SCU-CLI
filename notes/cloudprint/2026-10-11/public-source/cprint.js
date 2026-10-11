$(function () {
    var set_filesize = Common.getCookie("set_filesize");
    if (set_filesize == "0" || set_filesize == "") {
        $(".filesize").addClass("none")
    } else {
        $("#filesize").text(set_filesize)
    }
    addCommon();
    Common.listenAjax();
    Cprint.check();
    $("nav li").eq(0).addClass("active");
    var cprintObj = {};
    cprintObj.regEvent = function () {
        $("#tf").on("change", ".attr-wrap input", function () {
            if (!$(this).parent().hasClass("pagestyle")) {
                $(this).siblings(".style-input").addClass("active");
                $(this).parent().siblings(".radio").find(".style-input").removeClass("active");
                if ($(this).parent().next().hasClass("none")) {
                    $(this).parent().next().removeClass("none");
                } else {
                    $(this).parent().siblings(".pagestyle").addClass("none");
                    $("#dwFrom").val(1);
					$("#dwTo").val(1);
                }
            }
            // $(this).parent().next().removeClass("none");
            // $(this).parent().siblings(".range").next().addClass("none");
        });
        $("#dwFrom").blur(function(){
			let a = $("#dwFrom").val();
			let b = $("#dwTo").val();
			if (parseInt(a) > parseInt(b)) {
				$(".mesbox").css("height", "110%");
                $("#dwTo").val($("#dwFrom").val());
			}
		})
		$("#dwTo").blur(function(){
			let a = $("#dwFrom").val();
			let b = $("#dwTo").val();
			if (parseInt(a) > parseInt(b)) {
				$(".mesbox").css("height", "110%");
				$("#dwFrom").val($("#dwTo").val());
			}
		})
        var file;
        $(".file-choose-btn").on("click", function () {
            $("#file").click();
        });
        $("#file").change(function () {
            var filetype;
            var flen;
            var filearr = $(this).val().split("\\");
            flen = filearr.length;
            filename = filearr[flen - 1];
            filetypearr = filename.split(".");
            var ftlen = filetypearr.length;
            filetype = filetypearr[ftlen - 1];
            var canarr = ['jpg', 'jpeg', 'heic', 'png', 'pdf', 'txt', 'doc', 'xls', 'docx', 'xslx', "ppt", "pptx", "wps", "wpt", "doc", "dot", "rtf", "docx", "dotx", "docm", "dotm", "xml", "mht", "mhtml", "html", "htm", "doc", "xml", "xps", "odt", "xlsx", "xlsm", "xlsb", "xls", "csv", "xltx", "xltm", "xlt", "xls", "prn", "dif", "slk", "xla", "xlam", "xps", "xlsx", "ods", "dps", "dpt", "ppt", "pot", "pps", "pptx", "pptm", "potm", "ppsm", "pptm"]
            var flag = false;
            for (var i = 0; i < canarr.length; i++) {
                if (canarr[i] == (filetype.toLowerCase())) {
                    flag = true;
                }
            }
            // if (filetype.toLowerCase() == 'pdf') {
            //     $("#dwPdfImage").attr("disabled", false);
            // } else {
            //     $("#dwPdfImage").attr("disabled", true);
            // }
            // 对于非office/pdf文档，不支持选择纸张大小
            var disableSize = false;
            if (filetype.toLowerCase() == 'jpg')
                disableSize = true;
            if (filetype.toLowerCase() == 'jpeg')
                disableSize = true;
            if (filetype.toLowerCase() == 'png')
                disableSize = true;
            if (filetype.toLowerCase() == 'txt')
                disableSize = true;
            if (filetype.toLowerCase() == 'jpg' || filetype.toLowerCase() == 'jpeg' || filetype.toLowerCase() == 'png') {
                $(".pagebox").addClass("none");
            } else {
                $(".pagebox").removeClass("none");
            }
            $("#dwPaperId256").attr("disabled", disableSize);
            if (disableSize == true) {
                if ($("#dwPaperId256").is(':checked') == true) {
                    $("#dwPaperId9").click();
                }
            }

            if (!flag) {
                if (hostlang == 2) {
                    Cprint.createShowMes("This format file is not supported");
                } else {
                    Cprint.createShowMes("不支持此种格式文件");
                }
                $(".filename").html("");
                $("#upfile").addClass("disabled").attr("disabled", true);
                return;
            }
            $(".filename").html(filename).css("visibility", "visible");
            // $(".file-choose-btn").hide();
            // $(".reselect-btn").show();
            $(".file-choose").addClass("active");
            $("#upfile").removeClass("disabled").attr("disabled", false);
        });
        //份数
        $(".handle-num").click(function () {
            var $dwCopies = $("#dwCopies");
            var num = Number($(this).attr("data-value"));
            var nowcopies = Number($dwCopies.val());
            if ((num == -1) && (nowcopies == 1)) {
                $(this).addClass("disabled");
            } else {
                $(this).removeClass("disabled");
            }
            var copies = nowcopies + num;
            if (copies >= 1) {
                $dwCopies.val(copies);
            }
        });
        var form = document.getElementById("tf");
        if (Common.IEVersion() < 12 && Common.IEVersion() > 0) {
            $("#tf").attr({
                "action": "/api/client/CloudPrint/Upload",
                "method": "post",
                "enctype": "multipart/form-data"
            });
        } else {
            $("#tf").on("submit", function (ev) {
                var oData = new FormData(form);
                //var oReq = new ActiveXObject("Microsoft.XMLHTTP");
                var oReq = new XMLHttpRequest();
                /*if(window.ActiveXObject){ //如果是IE浏览器   
                    console.log("wo shi IE")				
                    oReq = new ActiveXObject("Microsoft.XMLHTTP");    
                }else if(window.XMLHttpRequest){ //非IE浏览器    
                    oReq = new XMLHttpRequest(); 
                    oReq = new ActiveXObject("Microsoft.XMLHTTP"); 					
                    console.log("wo bushi IE")					
                }*/
                if (oData.get("dwFrom") == 0) {
                    oData.set("dwTo", 0);
                } else {
                    oData.set("dwFrom", $("#dwFrom").val());
                    oData.set("dwTo", $("#dwTo").val())
                }
                oReq.open("POST", host + "/client/CloudPrint/Upload", true);
                oReq.onload = function () {
                    $("#upfile").removeClass("active");
                    $(".loadbtn").hide();
                    if (oReq.status == 200) {
                        var reg = new RegExp("(^|&)" + "code" + "=([^&]*)(&|$)");
                        var query;
                        if (!oReq.responseURL) {
                            query = "http://cprint.unifound.net/client/new/cprintPc/printDoc.html?t=1618283864344&code=0&message=";
                        } else {
                            query = oReq.responseURL.split("?")[1].match(reg);
                        }
                        var code = 0;
                        if (query != null)
                            code = query[2];
                        var message = oReq.responseURL.split("message=")[1];
                        if (code == 0) {
                            Cprint.createSuc(hostlang == 2 ? "Upload success" : "上传成功", after);
                        } else {
                            if (message != "") {
                                Cprint.createShowMes(decodeURI(message) + (hostlang == 2 ? "[Please use driver]" : "【请尝试驱动提交】"));
                            } else {
                                Cprint.createShowMes(hostlang == 2 ? "Print failed" : "打印失败【请尝试驱动提交】");
                            }
                        }
                        //                        if (hostlang == 2) {
                        //                            Cprint.createSuc("Upload success", after);
                        //                        } else {
                        //                            Cprint.createSuc("上传1成功", after);
                        //                        }
                        function after() {
                            var gourl = window.location.href.split("/");
                            gourl[gourl.length - 1] = "printDoc.html";
                            window.location.href = gourl.join("/");
                        }
                    } else {
                        if (oReq.status == 413) {
                            if (hostlang == 2) {
                                Cprint.createShowMes("File too large upload failure");
                            } else {
                                Cprint.createShowMes("文件过大上传失败");
                            }
                        } else {
                            if (hostlang == 2) {
                                Cprint.createShowMes("Upload failure");
                            } else {
                                Cprint.createShowMes("上传失败");
                            }
                        }
                    }
                };
                oReq.onerror = function () {
                    $("#upfile").removeClass("active");
                    $(".loadbtn").hide();
                    // if(oReq.messages)
                    // if (hostlang == 2) {
                    //     Cprint.createShowMes("Upload failure (Network error)");
                    // } else {
                    //     Cprint.createShowMes("上传失败(网络错误)");
                    // }
                };
                oReq.send(oData);
                ev.preventDefault();
            });
        }

        // 上传文件
        $("#upfile").on("click", function (e) {
            $(this).addClass("active");
            $(".loadbtn").show();
        });
        $("#dwCopies").on("input", function () {
            var len = $(this).val().length;
            if (len > 3) {
                $(this).css("width", (0.66 + (len - 3) * 0.14) + "rem");
            }
        });
    };
    cprintObj.getData = function () {
        Cprint.judgePrintColor();
        Cprint.judgePrintType();
        $("#dwCopies").val(1);
        Cprint.switchLang();
        var gourl = window.location.href.split("/");
        gourl[gourl.length - 1] = "printDoc.html";
        // $(".filename").html("备注：未上传附件").css("visibility", "visible");
        // $(".file-choose").removeClass("active");
        // $("#upfile").addClass("disabled").attr("disabled", true);
        // window.location.href = "./printDoc.html";
        $("#BackURL").val(Common.convertURL(gourl.join("/")));
    };
    cprintObj.regEvent();
    cprintObj.getData();
});