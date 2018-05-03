// Kernel specific extension for lgo.
// http://jupyter-notebook.readthedocs.io/en/stable/extending/frontend_extensions.html#kernel-specific-extensions

define(function(){
  var formatCells = function () {
    var cells = Jupyter.notebook.get_selected_cells();
    for (var i = 0; i < cells.length; i++) {
      (function(){
        var editor = cells[i].code_mirror;
        var msg = {code: editor.getValue()};
        var cb = function(msg) {
          if (!msg || !msg.content || msg.content.status != 'ok') {
            // TODO: Show an error message.
            return;
          }
          editor.setValue(msg.content.code);
        };
        Jupyter.notebook.kernel.send_shell_message("gofmt_request", msg, {shell: {reply: cb}});
      })();
    }
  };

  var action = {
    icon: 'fa-align-left', // a font-awesome class used on buttons, etc
    help    : 'Format Go',
    handler : formatCells
  };
  var prefix = 'lgo-kernel';
  var actionName = 'format-code';

  var fullActionName = Jupyter.actions.register(action, actionName, prefix);
  Jupyter.toolbar.add_buttons_group([fullActionName]);

  return {
    onload: function(){}
  }
});
