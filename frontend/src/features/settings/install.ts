import type { ToolProgram } from './types'

/** The commands that install a program: apt on Ubuntu (null when Ubuntu has no package) and Homebrew on macOS. */
export function installCommands(program: Pick<ToolProgram, 'apt' | 'brew'>): { apt: string | null; brew: string } {
  return {
    apt: program.apt ? `sudo apt install ${program.apt}` : null,
    brew: `brew install ${program.brew}`,
  }
}
