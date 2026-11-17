import ast
import gc
import sys
import weakref


observed = []


class SignalText:
    def __init__(self):
        observed.append(weakref.ref(self))

    def get_text(self):
        return "ni"


def check(source_path):
    with open(source_path, encoding="utf-8") as source:
        tree = ast.parse(source.read())
    printed = []
    namespace = {"events": [], "signal_texts": [], "print": lambda *args, **kwargs: printed.append(list(args))}
    definitions = ast.Module(body=[node for node in tree.body if isinstance(node, ast.FunctionDef)], type_ignores=[])
    exec(compile(ast.fix_missing_locations(definitions), source_path, "exec"), namespace)
    callbacks = {}
    for node in ast.walk(tree):
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Attribute) and node.func.attr == "connect":
            if isinstance(node.args[0], ast.Constant) and isinstance(node.args[1], ast.Lambda):
                callbacks[node.args[0].value] = eval(compile(ast.Expression(node.args[1]), source_path, "eval"), namespace)
    fcitx = "update-preedit-text-with-mode" in callbacks
    cases = [
        ("update-preedit-text", (2, True), ["update" if fcitx else "reference preedit", "ni", 2, True]),
        ("commit-text", (), ["commit" if fcitx else "reference commit", "ni"]),
    ]
    if fcitx:
        cases.append(("update-preedit-text-with-mode", (2, True, 1), ["mode", "ni", 2, True, 1]))
    for signal, tail, expected in cases:
        namespace["events"].clear()
        printed.clear()
        callbacks[signal](None, SignalText(), *tail)
        gc.collect()
        if observed[-1]() is None:
            raise AssertionError(f"{source_path}/{signal}: native signal text released before emitter returned")
        if (namespace["events"] or printed) != [expected]:
            raise AssertionError(f"{source_path}/{signal}: reference observation changed")
        namespace["signal_texts"].clear()
        gc.collect()
        if observed[-1]() is not None:
            raise AssertionError(f"{source_path}/{signal}: native signal text leaked after dispatch")
        print(f"{source_path}/{signal}: retained through emitter, released after dispatch")


for source_path in sys.argv[1:]:
    check(source_path)
