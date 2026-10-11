$(function () {
    addCommon();
    $("nav li").eq(1).addClass("active");
    Cprint.check();
    Cprint.createDelModel();
    Common.listenAjax();
    var $printDoc = $("#printDoc");
    var pDocObj = {};
    pDocObj.regEvent = function () {
        var $handleBtn = $(".handle-btn");//下载或者删除按钮
        var $allSelectBtn = $("#all-select-btn");//全选
        // 全选
        $printDoc.on("change", "#all-select-btn", function () {
            var $inputs = $("input[type=checkbox]");//页面所有可选项
            if ($(this).prop("checked")) {
				for (var i = $inputs.length; i >= 0; i--) {
					$($inputs[i]).prop("checked", true);
                };
				$handleBtn.removeClass("disabled");
            } else {
				for (var i = $inputs.length; i >= 0; i--) {
					$($inputs[i]).prop("checked", false);
                };
                $handleBtn.addClass("disabled");
            }
            $("#select-num").html($("input[name='scandoc']:checked").length);
        });
        // 每项
        $printDoc.on("click", "input[name='scandoc']", function () {
            var alllength = $("input[name='scandoc']").length;
            var checkedlength = $("input[name='scandoc']:checked").length;
            if (alllength == checkedlength) {
                $allSelectBtn.attr("checked", true);
            } else {
                $allSelectBtn.attr("checked", false);
            }
            if (checkedlength > 0) {
                $handleBtn.removeClass("disabled");
            } else {
                $handleBtn.addClass("disabled");
            }
            $("#select-num").html(checkedlength);
        });

        // 删除
        $printDoc.on("click", "#del-btn", function () {
            if ($(this).hasClass("disabled")) {
                if (hostlang == 2) {
                    Cprint.createSuc("please select");
                } else {
                    Cprint.createSuc("请选择要删除的文件");
                }
                return;
            }
            $(".modelwrap").show();
            // $("body").addClass("over_h");
        });
        // 确认删除
        $("body").on("click", ".confirm-btn", function () {
            $("input[name='scandoc']:checked").each(function (index) {
                delScan($(this).attr("data-dwJobId"), $(this));
            });

            function delScan(dwJobId, $target) {
                var option = {
                    dwJobId: dwJobId,
                    dwOldJobId: dwJobId
                };
                $.ajax({
                    url: host + '/client/PrintJob/Del',
                    type: 'post',
                    data: JSON.stringify(option),
                    dataType: 'JSON',
                    contentType: 'application/json',
                    success: function (redata) {
                        if (!Cprint.returnAjax(redata)) {
                            return;
                        }
                        $target.parent().parent().remove();
                        //删除成功
                        pDocObj.getData();
                    },
                    complete: function (e) {
                        Cprint.complete(e);
                    },
                    error: function (redata) {
                        Cprint.errorAjax();
                    }
                });
            }
        });

        // 页面滚动操作条固定
        var sclflag = $(".main").offset().top + 50;
        $(document).scroll(function () {
            var scroH = $(document).scrollTop();  //滚动高度
            if (scroH > sclflag) {  //距离顶部大于100px时
                $("#footbar").addClass("active_fixed").width($(".tab-page").width());
                $(".main").addClass("fixed");

            } else {
                $("#footbar").removeClass("active_fixed");
                $(".main").removeClass("fixed");

            }
        });

        var dwJobId;
		var dwCreateTime;
        var pagenum = 1;
        var total;
        // 预览
        $("body").on("click", ".pre_js", function () {
            var $this = $(this);
            $("#pre").fadeIn();
            dwJobId = $this.attr("data-dwJobId");
			dwCreateTime = $this.attr("data-dwCreateTime");
            total = $this.attr("data-pages");
            if (total <= 1) {
                $(".handlebtn").hide();
            } else {
                $(".handlebtn").show();
            }
            $("#totalnum").html(total);
			$("#preimg").attr("src", "./image/loading.gif");
            pagenum = 1;
            getPreviewPage();
        });
        // 预览弹窗
        // 关闭
        $("#pre").on("click", ".close-btn,.maskbg", function () {
            $("#pre").fadeOut();
        });
        // 下一页上一页
        $("#pre").on("click", ".nextbtn", function () {
            if (pagenum == total) {
                $(this).addClass("disabled");
                $("#pre .beforebtn").removeClass("disabled");
            } else if (pagenum > total) {
                return;
            } else if(pagenum < total){
				pagenum = pagenum + Number($(this).attr("data-type"));
			} else {
                $("#pre .beforebtn").removeClass("disabled");
                $(this).removeClass("disabled");
            }
            getPreviewPage();
        });
        // 下一页上一页
        $("#pre").on("click", ".beforebtn", function () {
            if (pagenum == 2) {
                $(this).addClass("disabled");
                $("#pre .nextbtn").removeClass("disabled");
            } else if (pagenum == 1) {
                return;
            } else {
                $("#pre .nextbtn").removeClass("disabled");
                $(this).removeClass("disabled");
            }
            pagenum--;
            getPreviewPage();
        });

        function getPreviewPage() {
            $.ajax({
                url: host + '/client/PrintJob/PreviewPage?timestamp=' + Math.random(),
                type: 'get',
                data: {
                    dwJobId: dwJobId,
					dwCreateTime: dwCreateTime,
                    pageNo: pagenum
                },
                success: function (redata) {
                    if (!Cprint.returnAjax(redata)) {
                        return;
                    }
                    var imgSrc = "data:image/png;base64," + redata.result;
                    $("#preimg").attr("src", imgSrc);
                    $("#nownum").html(pagenum);
                },
                complete: function (e) {
                    Cprint.complete(e);
                },
                error: function (redata) {
                    Cprint.errorAjax();
                }
            });
        }
    };
    pDocObj.getData = function () {
        var cw=$(window).width();
        var cutlen =10;
        if(cw<399){
            cutlen=6;
        }else if(cw<499){
            cutlen=7;
        }else if(cw<599){
            cutlen=10;
        }else if(cw<799){
            cutlen=12;
        }else if(cw<999){
            cutlen=18;
        }else if(cw<1300){
            cutlen=22;
        }else{
            cutlen=24;
        }
        // 获取打印列表
        $.ajax({
            url: host + '/client/PrintJob/Get?timestamp=' + Math.random(),
            type: 'get',
            success: function (redata) {
                if (!Cprint.returnAjax(redata)) {
                    return;
                }
                if ((!redata.result) || (redata.result.length == 0)) {
                    $(".print-list").hide();
                    $(".nolist").show();
                    $("#footbar").hide();
                    return;
                }
                $("#footbar").show();
                var result = redata.result;
                for (var i = 0; i < result.length; i++) {
                    result[i].szPaperDetail = JSON.parse(result[i].szPaperDetail);
                    // 文件类型
                    result[i].x_doctype = Cprint.getDocType(result[i].szJobName);
                    // 文件名称处理
                    var namearr = result[i].szJobName.split(".");
                    var namelen = namearr.length - 1;
                    namearr[namearr.length - 1]="";
                    result[i].x_szJobName = Common.cut(namearr.join("."),cutlen)+""+result[i].x_doctype.type;
                    //时间
                    result[i].x_dwCreateDate = Common.fomateDate(result[i].dwCreateDate, ".");
                    result[i].x_dwCreateTime = Common.fommateTimeToHHMM(result[i].dwCreateTime, ":");
                    for (var j = 0; j < result[i].szPaperDetail.length; j++) {
                        // 纸型======================根据查询出的可用纸型匹配出纸型类型
                        if (result[i].szPaperDetail[j].dwPaperID == 9) {
                            result[i].szPaperDetail[j].x_dwPaperID = "A4";
                        } else {
                            result[i].szPaperDetail[j].x_dwPaperID = "A3";
                        }
                        result[i].x_numArr = [];
                        let num = result[i].szPaperDetail[j].dwBWPages + result[i].szPaperDetail[j].dwColorPages
                        for (var k = 0; k < num; k++) {
                            result[i].x_numArr.push(k);
                        }
                    }
                }
                redata.result = Common.turnArr(redata.result);
                $("#printul").html(Common.bindTemplate("plist-template", redata));
                // 中英文
                Cprint.switchLang();
                $("#select-num").html($("input[name='scandoc']:checked").length);
            },
            complete: function (e) {
                Cprint.complete(e);
            },
            error: function (redata) {
                Cprint.errorAjax();
            }
        });
    };
    pDocObj.getData();
    pDocObj.regEvent();
});