/**
 * Interaction layer — Gesture Layer Architecture (production-grade).
 * Decouples gesture handling from validation, animation, and network dispatch.
 */
export { validateCardDrop } from "./interaction.engine";
export type { DropResult, ValidateDropContext } from "./interaction.engine";
export { detectZone } from "./zones";
export type { DropZone, Point, TableRect } from "./zones";
export { isValidMove } from "./validation";
export type { ValidationResult } from "./validation";
