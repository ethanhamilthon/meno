# session
- session.models({provider?, refresh?}) returns {models, errors}. It lists the models of the enabled providers and what is known about them (efforts, contextWindow, images, prices; null means unknown). Some providers can fail while others succeed.
A subagent cannot compact the context.
