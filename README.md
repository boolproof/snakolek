# Snakolek
Snakolek is a simple text terminal snake game clone made with **Go**. This is the first thing I did with Go, back in 2018. I thought snake game is a nice exercise to learn a new language. Snakolek is relying on the great [termbox-go library](https://github.com/nsf/termbox-go), thanks to which, I have successfully ran the game on Linux, MacOS and Windows systems without any hassle.

![snakolek](http://snakolek.ironsys.pl/img/snakolek/snakolek.png)

The terminal app connects to http API to post/fetch highscores (and also display information about e.g. new version released). The binary files you can find on http://snakolek.ironsys.pl/download are compiled with secret that allows signing posted highscores so that they can be accepted by highscores API. You can see the all time highscores on http://snakolek.ironsys.pl/high-scores (the top one is not a hack, but a tribute to one of the greatest developers and most inspiring people I had opportunity to work with - [@voituk](https://github.com/voituk))

Although I haven't touched this code since august 2018, I just decided to publish it now, on new year's day of 2021. It's pretty rough, but let's keep it like that. I'm not planning to work on this anymore. But hey! As far as I remember, it works pretty well!

## Signed highscores posting
I'm not revealing the secret string that is used to sign highscores posted to http://snakolek.ironsys.pl/api/high-scores, because that would defeat the whole idea of public highscores list instantly, but I assume that it is possible, that someone skilled and experienced will be able to extract original secret string from available "official" binaries, especially now, having the source code. **If you manage to do that (discover the secret string for signature in a non brute-force way), I'd be happy to hear from you to learn how you approached it and achieved it.**

Anyway, if you pass correct value of `flc` var from `main` package on build, like that:

`go build -ldflags "-X main.flc=S0M3sEcR3T"`

the highscores posted by the built binary will be accepted by the  API endpoint mentioned above. If you won't pass the secret or pass the wrong value, you will get an error message when posting the score (binary will still be build, you will be able to play the game and fetch highscores from within the game). 


