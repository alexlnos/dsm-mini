// The window DSM opens from its main menu.
//
// It is deliberately thin: a DSM app is written in Ext JS against an internal
// framework that is neither documented nor stable between releases, so the
// only thing written against it here is a frame. Everything a person sees is
// an ordinary page in index.html, which is also what makes it testable in a
// normal browser.
//
// "allUsers": false in config keeps the icon out of a non-administrator's
// menu, but that is a courtesy, not a guard: DSM serves /webman/3rdparty/ to
// anyone, so the service checks the session on every request of its own.
Ext.ns("DSMMINI.Settings");

Ext.define("DSMMINI.Settings.AppInstance", {
    extend: "SYNO.SDS.AppInstance",
    appWindowName: "DSMMINI.Settings.AppWindow",
    constructor: function () {
        this.callParent(arguments);
    }
});

Ext.define("DSMMINI.Settings.AppWindow", {
    extend: "SYNO.SDS.AppWindow",
    constructor: function (config) {
        this.callParent([Ext.apply({
            resizable: true,
            maximizable: true,
            minimizable: true,
            width: 760,
            height: 720,
            minWidth: 420,
            minHeight: 480,
            layout: "fit",
            border: false,
            items: [{
                xtype: "box",
                itemId: "appframe",
                autoEl: {
                    tag: "iframe",
                    src: "/webman/3rdparty/dsm-mini/index.html",
                    frameborder: "0",
                    style: "width:100%; height:100%; border:none;"
                }
            }]
        }, config)]);
    }
});
