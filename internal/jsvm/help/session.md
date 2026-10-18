# session
- session.models({provider?, refresh?}) returns {models, errors}. It lists the models of the enabled providers and what is known about them (efforts, contextWindow, images, prices; null means unknown). Some providers can fail while others succeed.
There is no way to compact the context from js. The application compacts it by itself when it reaches the set share of the context window, and the user can run :compact.
