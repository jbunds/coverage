[scc]:    https://github.com/boyter/scc
[cocomo]: https://en.wikipedia.org/wiki/COCOMO

#### code metrics via [`scc`][scc]

see also [COCOMO][cocomo] on Wikpedia

---

```
$ date
Wed Apr 15 11:08:38 CEST 2026
```

```
$ scc --exclude-dir .git --include-ext go,css,js,mjs,yml,yaml --dryness --by-file --wide
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Language                              Files     Lines   Blanks  Comments     Code Complexity Complexity/Lines
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
YAML                                     12       421       50        36      335          0             0.00
(ULOC)                                            266
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
.github/workflows/pages.yml                        82        7        13       62          0             0.00
action.yml                                         64        7         8       49          0             0.00
.github/workflows/test-go.yml                      50        9         1       40          0             0.00
.pre-commit-config.yaml                            37        1         2       34          0             0.00
.github/workflows/lint-go.yml                      35        7         2       26          0             0.00
.github/actions/upload-pages/action.yml            30        5         6       19          0             0.00
.github/workflows/lint-js.yml                      26        6         0       20          0             0.00
.github/workflows/lint-css.yml                     26        3         0       23          0             0.00
~rkflows/validate-actions-and-workflows.yml        24        2         0       22          0             0.00
.github/actions/setup-node/action.yml              19        2         1       16          0             0.00
.golangci.yml                                      16        0         2       14          0             0.00
.github/dependabot.yml                             12        1         1       10          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Go                                        8      1968      169        72     1727        272           121.35
(ULOC)                                           1109
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
main_test.go                                      711       30         2      679         76            11.19
main.go                                           495       70        48      377         92            24.40
tree_test.go                                      232        7         0      225         28            12.44
tree.go                                           190       32        11      147         32            21.77
flags_test.go                                     180        4         0      176         24            13.64
progress/progress.go                              110       20         9       81         11            13.58
flags.go                                           40        2         1       37          9            24.32
progress/progress_test_stub.go                     10        4         1        5          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
JavaScript                                5       333       33        51      249          9            19.87
(ULOC)                                            249
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
demo/render.js                                    184       24        39      121          0             0.00
demo/helpers.js                                   111        5        11       95          7             7.37
validate.js                                        19        3         0       16          2            12.50
demo/eslint.config.js                              15        1         0       14          0             0.00
stylelint.config.mjs                                4        0         1        3          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
CSS                                       2       254       38         1      215          0             0.00
(ULOC)                                            164
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
css/style.css                                     135       18         0      117          0             0.00
css/tree.css                                      119       20         1       98          0             0.00
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Total                                    27      2976      290       160     2526        281           141.22
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Unique Lines of Code (ULOC)                      1781
DRYness %                                        0.60
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
Estimated Cost to Develop (organic) $71,474
Estimated Schedule Effort (organic) 5.05 months
Estimated People Required (organic) 1.26
Processed 92253 bytes, 0.092 megabytes (SI)
─────────────────────────────────────────────────────────────────────────────────────────────────────────────
```
