// The arrival card: once a player has an archetype and is dressed, before they go online, a card says who they
// are (badge, name, the archetype's story from stories.ts), what to do first and how to move. The player stands
// facing the camera meanwhile (play.ts setDressing), like in the dressing panel.
import { el } from './hud';
import type { Archetype } from './archetypes';
import type { Story } from './stories';

export interface StoryHandlers {
  /** the player read it (button, Enter / Space on it, or Esc) */
  done(): void;
}

export class StoryCard {
  private readonly root = el('story');
  private readonly badge = el('story-badge');
  private readonly title = el('story-title');
  private readonly opening = el('story-opening');
  private readonly todo = el<HTMLOListElement>('story-todo');
  private readonly tip = el('story-tip');
  private readonly go = el<HTMLButtonElement>('story-go');

  constructor(private readonly on: StoryHandlers) {
    this.go.addEventListener('click', () => this.dismiss());
  }

  get isOpen(): boolean {
    return !this.root.hidden;
  }

  open(a: Archetype, story: Story): void {
    this.badge.textContent = a.badge.short;
    this.badge.style.background = a.badge.color;
    this.badge.style.color = a.badge.text;
    this.title.textContent = a.name.vi;
    this.opening.textContent = story.opening;
    this.todo.replaceChildren(
      ...story.todo.map((t) => {
        const li = document.createElement('li');
        li.textContent = t;
        return li;
      }),
    );
    this.tip.textContent = story.tip;
    this.root.hidden = false;
    document.body.classList.add('is-story');
    this.go.focus({ preventScroll: true });
  }

  /** Close and tell the game to carry on. */
  dismiss(): void {
    if (!this.isOpen) return;
    this.close();
    this.on.done();
  }

  /** Close without the callback (leaving play). */
  close(): void {
    this.root.hidden = true;
    document.body.classList.remove('is-story');
  }
}
